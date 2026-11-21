package services

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"subdeck/internal/database"
	"subdeck/internal/models"
	"subdeck/internal/parser"
)

const maxSubscriptionBodyBytes = 10 << 20 // 10 MiB

type inflightFetch struct {
	done  chan struct{}
	nodes []*models.Node
	err   error
}

type FetcherService struct {
	db                  *database.DB
	AllowPrivateNetwork bool
	secureClient        *http.Client
	insecureClient      *http.Client
	privateClient       *http.Client
	privateInsecure     *http.Client

	inflightMu sync.Mutex
	inflight   map[string]*inflightFetch
}

// IsBlockedIP reports whether an IP falls in loopback, private RFC1918/ULA, link-local, multicast, or unspecified ranges.
func IsBlockedIP(ip net.IP) bool {
	if ip == nil {
		return true
	}
	return ip.IsLoopback() ||
		ip.IsPrivate() ||
		ip.IsLinkLocalUnicast() ||
		ip.IsLinkLocalMulticast() ||
		ip.IsMulticast() ||
		ip.IsUnspecified()
}

func newSafeHTTPClient(insecure bool, allowPrivate bool) *http.Client {
	dialer := &net.Dialer{
		Timeout:   10 * time.Second,
		KeepAlive: 30 * time.Second,
	}

	transport := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: insecure},
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			if allowPrivate {
				return dialer.DialContext(ctx, network, addr)
			}
			host, port, err := net.SplitHostPort(addr)
			if err != nil {
				return nil, err
			}
			ips, err := net.DefaultResolver.LookupIPAddr(ctx, host)
			if err != nil {
				return nil, err
			}
			if len(ips) == 0 {
				return nil, fmt.Errorf("DNS lookup returned no addresses for %s", host)
			}
			for _, ipAddr := range ips {
				if IsBlockedIP(ipAddr.IP) {
					return nil, fmt.Errorf("blocked connection to private/reserved IP %s (%s)", ipAddr.IP.String(), host)
				}
			}
			// Connect directly to the first verified public IP to avoid DNS rebinding TOCTOU
			return dialer.DialContext(ctx, network, net.JoinHostPort(ips[0].IP.String(), port))
		},
	}

	return &http.Client{
		Transport: transport,
		Timeout:   20 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 5 {
				return errors.New("stopped after 5 redirects")
			}
			scheme := strings.ToLower(req.URL.Scheme)
			if scheme != "http" && scheme != "https" {
				return fmt.Errorf("redirect to disallowed scheme %q", scheme)
			}
			return nil
		},
	}
}

func NewFetcherService(db *database.DB) *FetcherService {
	return &FetcherService{
		db:              db,
		secureClient:    newSafeHTTPClient(false, false),
		insecureClient:  newSafeHTTPClient(true, false),
		privateClient:   newSafeHTTPClient(false, true),
		privateInsecure: newSafeHTTPClient(true, true),
		inflight:        make(map[string]*inflightFetch),
	}
}

func (s *FetcherService) recordFetchError(sub *models.Subscription, errMsg string) {
	failures, err := s.db.MarkSubscriptionFetchError(sub.ID, errMsg)
	if err != nil {
		return
	}
	// Trigger alert on the 3rd consecutive failure and periodically thereafter to avoid alert storms
	if failures == 3 || (failures > 3 && failures%12 == 0) {
		settings := s.db.GetSystemSettings()
		if settings != nil && settings.WebhookURL != "" {
			title := fmt.Sprintf("SubDeck 订阅拉取告警: %s", sub.Name)
			msg := fmt.Sprintf("远程订阅 [%s] 已连续 %d 次拉取失败，当前保留上次有效缓存 (%d 节点)。\n错误详情: %s",
				sub.Name, failures, sub.NodeCount, errMsg)
			go func(url, t, m string) {
				_ = SendWebhookNotification(url, "subscription_fetch_failed", t, m)
			}(settings.WebhookURL, title, msg)
		}
	}
}

// FetchSubscription pulls content from the subscription URL (for remote) or parses RawContent (for local).
// Concurrent pulls for the same subID are deduplicated via an inflight singleflight group.
func (s *FetcherService) FetchSubscription(subID string) ([]*models.Node, error) {
	s.inflightMu.Lock()
	if call, ok := s.inflight[subID]; ok {
		s.inflightMu.Unlock()
		<-call.done
		return call.nodes, call.err
	}
	call := &inflightFetch{done: make(chan struct{})}
	s.inflight[subID] = call
	s.inflightMu.Unlock()

	defer func() {
		s.inflightMu.Lock()
		delete(s.inflight, subID)
		s.inflightMu.Unlock()
		close(call.done)
	}()

	call.nodes, call.err = s.fetchSubscriptionOnce(subID)
	return call.nodes, call.err
}

func (s *FetcherService) fetchSubscriptionOnce(subID string) ([]*models.Node, error) {
	sub, err := s.db.GetSubscription(subID)
	if err != nil {
		return nil, fmt.Errorf("subscription not found: %w", err)
	}

	if sub.Type == models.SubscriptionTypeLocal {
		nodes, err := parser.ParseSubscriptionContent([]byte(sub.RawContent))
		if err != nil {
			s.recordFetchError(sub, fmt.Sprintf("parse failed: %v", err))
			return nil, err
		}
		for _, n := range nodes {
			n.Source = sub.Name
		}
		if err := s.db.UpdateSubscriptionCache(sub.ID, nodes, "ok", ""); err != nil {
			return nil, fmt.Errorf("failed to save cache: %w", err)
		}
		return nodes, nil
	}

	// Remote subscription: fetch every configured URL, then commit one combined cache.
	urls := sub.EffectiveURLs()
	if len(urls) == 0 {
		errMsg := "invalid remote subscription URL: only http/https URLs with host are allowed"
		s.recordFetchError(sub, errMsg)
		return nil, errors.New(errMsg)
	}

	allowPrivate := s.AllowPrivateNetwork
	if !allowPrivate {
		if sysSettings := s.db.GetSystemSettings(); sysSettings != nil && sysSettings.AllowPrivateFetch {
			allowPrivate = true
		}
	}

	client, err := s.clientForSubscription(sub, allowPrivate)
	if err != nil {
		s.recordFetchError(sub, err.Error())
		return nil, err
	}

	type urlResult struct {
		nodes    []*models.Node
		userInfo string
	}
	results := make([]urlResult, len(urls))
	failures := make([]string, len(urls))

	// Bounded concurrency for multi-URL sources; a single URL runs inline-fast.
	maxConcurrency := 4
	sem := make(chan struct{}, min(maxConcurrency, len(urls)))
	var wg sync.WaitGroup
	for i, rawURL := range urls {
		wg.Add(1)
		sem <- struct{}{}
		go func(idx int, target string) {
			defer wg.Done()
			defer func() { <-sem }()
			nodes, info, fetchErr := s.fetchRemoteURL(sub, target, client)
			if fetchErr != nil {
				failures[idx] = fetchErr.Error()
			} else {
				results[idx] = urlResult{nodes: nodes, userInfo: info}
			}
		}(i, rawURL)
	}
	wg.Wait()

	failed := make([]string, 0, len(urls))
	for i, f := range failures {
		if f != "" {
			failed = append(failed, fmt.Sprintf("URL#%d: %s", i+1, f))
		}
	}

	skip := sub.FailurePolicy == models.FetchFailureSkip
	if len(failed) > 0 && (!skip || len(failed) == len(urls)) {
		errMsg := fmt.Sprintf("%d/%d URL(s) failed: %s", len(failed), len(urls), strings.Join(failed, "; "))
		s.recordFetchError(sub, errMsg)
		return nil, errors.New(errMsg)
	}

	var combined []*models.Node
	firstUserInfo := ""
	accepted := 0
	for i := range urls {
		if failures[i] != "" {
			continue
		}
		combined = append(combined, results[i].nodes...)
		if firstUserInfo == "" {
			firstUserInfo = results[i].userInfo
		}
		accepted++
	}
	if len(combined) == 0 {
		errMsg := fmt.Sprintf("no nodes parsed from %d URL(s): %s", len(urls), strings.Join(failed, "; "))
		s.recordFetchError(sub, errMsg)
		return nil, errors.New(errMsg)
	}

	status := "ok"
	errMsg := ""
	if len(failed) > 0 {
		// skip policy with partial success: commit successful URLs, surface the rest
		status = "partial"
		errMsg = fmt.Sprintf("%d/%d URL(s) failed (kept last-success URLs only): %s", len(failed), len(urls), strings.Join(failed, "; "))
	}

	// A single quota header cannot describe several unrelated accounts; for
	// multi-URL sources adopt the first successful header, never sum duplicates.
	clearUserInfo := len(urls) > 1 && firstUserInfo == ""
	if err := s.db.UpdateSubscriptionCacheWithOptions(sub.ID, combined, status, errMsg, firstUserInfo, clearUserInfo); err != nil {
		return nil, fmt.Errorf("failed to save cache: %w", err)
	}

	return combined, nil
}

// fetchRemoteURL fetches and parses one remote subscription URL. It returns the
// parsed nodes and the upstream Subscription-Userinfo header without touching the cache.
func (s *FetcherService) fetchRemoteURL(sub *models.Subscription, rawURL string, client *http.Client) ([]*models.Node, string, error) {
	parsedURL, err := url.ParseRequestURI(strings.TrimSpace(rawURL))
	if err != nil || (parsedURL.Scheme != "http" && parsedURL.Scheme != "https") || parsedURL.Host == "" {
		return nil, "", errors.New("invalid remote subscription URL: only http/https URLs with host are allowed")
	}

	req, err := http.NewRequest("GET", parsedURL.String(), nil)
	if err != nil {
		return nil, "", err
	}

	ua := sub.UserAgent
	if ua == "" {
		ua = "ClashMeta/v1.18.0 (SubDeck)"
	}
	req.Header.Set("User-Agent", ua)
	req.Header.Set("Accept", "*/*")

	resp, err := client.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("HTTP request failed: %s", sanitizeFetchError(err.Error(), sub.FetchProxyURL))
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, "", fmt.Errorf("server returned status: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxSubscriptionBodyBytes+1))
	if err != nil {
		return nil, "", fmt.Errorf("read body failed: %v", err)
	}
	if len(body) > maxSubscriptionBodyBytes {
		return nil, "", errors.New("subscription response exceeds 10MB limit")
	}

	nodes, err := parser.ParseSubscriptionContent(body)
	if err != nil {
		return nil, "", fmt.Errorf("parse failed: %v", err)
	}
	if len(nodes) == 0 {
		return nil, "", errors.New("no nodes parsed from response")
	}

	for _, n := range nodes {
		n.Source = sub.Name
	}
	return nodes, strings.TrimSpace(resp.Header.Get("Subscription-Userinfo")), nil
}

// FetchTemplate downloads a remote config template (Clash YAML / sing-box JSON)
// through the safe HTTP client. Only used on admin save/manual refresh — public
// export requests never trigger template fetches.
func (s *FetcherService) FetchTemplate(rawURL string, allowPrivate bool) ([]byte, error) {
	parsedURL, err := url.ParseRequestURI(strings.TrimSpace(rawURL))
	if err != nil || (parsedURL.Scheme != "http" && parsedURL.Scheme != "https") || parsedURL.Host == "" {
		return nil, errors.New("模板地址必须为合法的 http/https URL")
	}

	client := s.secureClient
	if allowPrivate {
		client = s.privateClient
	}

	req, err := http.NewRequest("GET", parsedURL.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "ClashMeta/v1.18.0 (SubDeck)")
	req.Header.Set("Accept", "*/*")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("模板下载失败: %s", sanitizeFetchError(err.Error(), ""))
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("模板下载失败: HTTP %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxSubscriptionBodyBytes+1))
	if err != nil {
		return nil, fmt.Errorf("模板读取失败: %v", err)
	}
	if len(body) > maxSubscriptionBodyBytes {
		return nil, errors.New("模板响应超过 10MB 限制")
	}
	return body, nil
}

// clientForSubscription picks the HTTP client honoring the per-subscription proxy,
// TLS-verification and private-network settings. A configured proxy must work or
// the fetch fails closed — never silently falling back to a direct connection.
func (s *FetcherService) clientForSubscription(sub *models.Subscription, allowPrivate bool) (*http.Client, error) {
	if strings.TrimSpace(sub.FetchProxyURL) != "" {
		proxyURL, err := ValidateProxyURL(sub.FetchProxyURL, allowPrivate)
		if err != nil {
			return nil, err
		}
		transport := &http.Transport{
			Proxy: http.ProxyURL(proxyURL),
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: sub.InsecureSkipVerify,
			},
		}
		return &http.Client{
			Transport: transport,
			Timeout:   30 * time.Second,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				if len(via) >= 5 {
					return errors.New("stopped after 5 redirects")
				}
				scheme := strings.ToLower(req.URL.Scheme)
				if scheme != "http" && scheme != "https" {
					return fmt.Errorf("redirect to disallowed scheme %q", scheme)
				}
				return nil
			},
		}, nil
	}

	if allowPrivate {
		if sub.InsecureSkipVerify {
			return s.privateInsecure, nil
		}
		return s.privateClient, nil
	}
	if sub.InsecureSkipVerify {
		return s.insecureClient, nil
	}
	return s.secureClient, nil
}

// validateProxyURL checks scheme and (unless private fetches are allowed) that the
// proxy endpoint itself is not a private/reserved address.
func ValidateProxyURL(raw string, allowPrivate bool) (*url.URL, error) {
	trimmed := strings.TrimSpace(raw)
	parsed, err := url.Parse(trimmed)
	if err != nil || parsed.Host == "" {
		return nil, errors.New("invalid fetch proxy URL")
	}
	scheme := strings.ToLower(parsed.Scheme)
	if scheme != "http" && scheme != "https" && scheme != "socks5" && scheme != "socks5h" {
		return nil, fmt.Errorf("unsupported fetch proxy scheme %q (http/https/socks5 only)", parsed.Scheme)
	}
	if !allowPrivate {
		hostname := strings.TrimSpace(parsed.Hostname())
		if strings.EqualFold(hostname, "localhost") {
			return nil, errors.New("fetch proxy points to a private address; enable the private-fetch setting to use it")
		}
		if ip := net.ParseIP(hostname); ip != nil && IsBlockedIP(ip) {
			return nil, errors.New("fetch proxy points to a private/reserved IP; enable the private-fetch setting to use it")
		}
	}
	return parsed, nil
}

// sanitizeFetchError strips proxy credentials from error messages that may embed
// the configured proxy URL before they reach SQLite, responses, logs or webhooks.
func sanitizeFetchError(msg, proxyURL string) string {
	if strings.TrimSpace(proxyURL) == "" {
		return msg
	}
	redacted := redactProxyURL(proxyURL)
	msg = strings.ReplaceAll(msg, proxyURL, redacted)
	if parsed, err := url.Parse(strings.TrimSpace(proxyURL)); err == nil && parsed.Host != "" {
		msg = strings.ReplaceAll(msg, parsed.Host, hostOnlyRedacted(parsed))
	}
	return msg
}

func redactProxyURL(raw string) string {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return "<proxy>"
	}
	if parsed.User != nil {
		parsed.User = url.User("REDACTED")
	}
	return parsed.String()
}

func hostOnlyRedacted(parsed *url.URL) string {
	if parsed.User != nil {
		return parsed.Host
	}
	return parsed.Host
}
