package handlers

import (
	"bytes"
	"fmt"
	"html/template"
	"net/http"
	"strings"

	"subdeck/internal/database"
	"subdeck/internal/generator"
	"subdeck/internal/models"
	"subdeck/internal/services"

	"github.com/gin-gonic/gin"
)

type ExportHandler struct {
	db *database.DB
}

func NewExportHandler(db *database.DB) *ExportHandler {
	return &ExportHandler{db: db}
}

type landingPageData struct {
	ProfileName  string
	NodeCount    int
	ProtoCounts  []protoCountItem
	BaseURL      string
	ClashFullURL string
	SingboxURL   string
	Base64URL    string
}

type protoCountItem struct {
	Label string
	Count int
}

var landingPageTmpl = template.Must(template.New("landing").Parse(`<!DOCTYPE html>
<html lang="zh-CN">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>{{.ProfileName}} - SubDeck 订阅中心</title>
  <style>
    :root {
      --bg: #090b0f;
      --card: #11141b;
      --border: rgba(255, 255, 255, 0.08);
      --text: #fafafa;
      --subtext: #a1a1aa;
      --primary: #2563eb;
      --primary-hover: #1d4ed8;
      --primary-text: #ffffff;
      --emerald: #10b981;
    }
    @media (prefers-color-scheme: light) {
      :root {
        --bg: #f6f7f9;
        --card: #ffffff;
        --border: rgba(15, 23, 42, 0.08);
        --text: #0f172a;
        --subtext: #64748b;
        --primary: #2563eb;
        --primary-hover: #1d4ed8;
        --primary-text: #ffffff;
        --emerald: #059669;
      }
    }
    * { box-sizing: border-box; margin: 0; padding: 0; }
    body {
      font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", "PingFang SC", Roboto, "Helvetica Neue", Arial, sans-serif;
      background: var(--bg);
      background-image: radial-gradient(rgba(113, 113, 122, 0.14) 1px, transparent 1px);
      background-size: 22px 22px;
      color: var(--text);
      min-height: 100dvh;
      display: flex;
      align-items: center;
      justify-content: center;
      padding: 1.5rem;
    }
    .card {
      background: var(--card);
      border: 1px solid var(--border);
      border-radius: 1rem;
      max-width: 520px;
      width: 100%;
      padding: 1.75rem;
      box-shadow: 0 10px 15px -3px rgba(0, 0, 0, 0.1);
    }
    .badge {
      display: inline-flex;
      align-items: center;
      background: rgba(161, 161, 170, 0.1);
      border: 1px solid var(--border);
      color: var(--text);
      padding: 0.2rem 0.5rem;
      border-radius: 0.375rem;
      font-size: 0.7rem;
      font-family: monospace;
      font-weight: 600;
      margin-bottom: 0.75rem;
    }
    h1 { font-size: 1.35rem; font-weight: 700; margin-bottom: 0.35rem; letter-spacing: -0.02em; }
    p.desc { font-size: 0.8rem; color: var(--subtext); margin-bottom: 1.25rem; line-height: 1.5; }
    .pills { display: flex; flex-wrap: wrap; gap: 0.4rem; margin-bottom: 1.25rem; }
    .pill {
      font-size: 0.7rem;
      font-weight: 600;
      font-family: monospace;
      background: rgba(161, 161, 170, 0.08);
      border: 1px solid var(--border);
      padding: 0.2rem 0.5rem;
      border-radius: 0.375rem;
      color: var(--subtext);
    }
    .section-title {
      font-size: 0.7rem;
      font-weight: 700;
      text-transform: uppercase;
      letter-spacing: 0.05em;
      color: var(--subtext);
      margin-bottom: 0.5rem;
    }
    .btn-group { display: flex; flex-direction: column; gap: 0.5rem; margin-bottom: 1.25rem; }
    .btn {
      display: flex;
      align-items: center;
      justify-content: space-between;
      padding: 0.65rem 0.85rem;
      border-radius: 0.5rem;
      font-size: 0.8rem;
      font-weight: 500;
      text-decoration: none;
      cursor: pointer;
      border: 1px solid var(--border);
      background: transparent;
      color: var(--text);
      transition: all 0.15s ease;
    }
    .btn:hover { background: rgba(161, 161, 170, 0.1); }
    .btn-primary {
      background: var(--primary);
      border-color: var(--primary);
      color: var(--primary-text) !important;
      font-weight: 600;
    }
    .btn-primary:hover { background: var(--primary-hover); }
    .toast {
      position: fixed;
      bottom: 2rem;
      background: var(--primary);
      color: var(--primary-text);
      padding: 0.5rem 1rem;
      border-radius: 0.5rem;
      font-size: 0.75rem;
      font-weight: 600;
      opacity: 0;
      transform: translateY(10px);
      transition: all 0.2s ease;
      pointer-events: none;
      box-shadow: 0 10px 15px -3px rgba(0,0,0,0.2);
    }
    .toast.show { opacity: 1; transform: translateY(0); }
  </style>
</head>
<body>
  <div class="card">
    <div class="badge">SubDeck 专属订阅</div>
    <h1>{{.ProfileName}}</h1>
    <p class="desc">该万能链接支持在任何主流客户端中自动识别并下发适配格式；您也可在下方一键复制专属配置链接。</p>

    <div class="pills">
      <span class="pill" style="color:var(--emerald);">有效节点: {{.NodeCount}}</span>
      {{range .ProtoCounts}}<span class="pill">{{.Label}}: {{.Count}}</span>{{end}}
    </div>

    <div class="section-title">万能自适应链接（直接复制填入客户端）</div>
    <div style="display:flex; gap:0.5rem; margin-bottom:1.5rem;">
      <input type="text" readonly value="{{.BaseURL}}" id="uniUrl" style="flex:1; padding:0.6rem 0.8rem; font-size:0.75rem; font-family:monospace; background:rgba(0,0,0,0.05); border:1px solid var(--border); border-radius:0.5rem; color:var(--text); outline:none;">
      <button type="button" class="btn btn-primary copy-btn" data-copy="{{.BaseURL}}" style="padding:0.6rem 1rem; font-size:0.75rem; white-space:nowrap;">复制</button>
    </div>

    <div class="section-title">专属格式一键复制</div>
    <div class="btn-group">
      <button type="button" class="btn copy-btn" data-copy="{{.ClashFullURL}}">
        <span>📋 复制 Clash / Mihomo 完整配置</span>
        <span style="font-size:0.75rem; color:var(--subtext);">YAML</span>
      </button>
      <button type="button" class="btn copy-btn" data-copy="{{.SingboxURL}}">
        <span>📋 复制 Sing-box 完整配置</span>
        <span style="font-size:0.75rem; color:var(--subtext);">JSON</span>
      </button>
      <button type="button" class="btn copy-btn" data-copy="{{.Base64URL}}">
        <span>📋 复制 Shadowrocket / V2RayN 通用订阅</span>
        <span style="font-size:0.75rem; color:var(--subtext);">Base64</span>
      </button>
    </div>

    <div class="section-title">直接下载配置文件</div>
    <div style="display:flex; gap:0.5rem;">
      <a href="{{.ClashFullURL}}" download class="btn" style="flex:1; justify-content:center; font-size:0.75rem;">下载 .yaml</a>
      <a href="{{.SingboxURL}}" download class="btn" style="flex:1; justify-content:center; font-size:0.75rem;">下载 .json</a>
      <a href="{{.Base64URL}}" download class="btn" style="flex:1; justify-content:center; font-size:0.75rem;">下载 .txt</a>
    </div>
  </div>

  <div class="toast" id="toast">已复制到剪贴板！</div>

  <script>
    function showCopyToast() {
      var toast = document.getElementById('toast');
      toast.classList.add('show');
      setTimeout(function () { toast.classList.remove('show'); }, 2000);
    }
    function fallbackCopy(text) {
      var ta = document.createElement('textarea');
      ta.value = text;
      ta.setAttribute('readonly', '');
      ta.style.position = 'fixed';
      ta.style.top = '-9999px';
      ta.style.left = '-9999px';
      document.body.appendChild(ta);
      ta.focus();
      ta.select();
      ta.setSelectionRange(0, text.length);
      var ok = false;
      try { ok = document.execCommand('copy'); } catch (e) {}
      document.body.removeChild(ta);
      return ok;
    }
    document.querySelectorAll('.copy-btn').forEach(function (btn) {
      btn.addEventListener('click', function () {
        var text = btn.getAttribute('data-copy') || '';
        if (navigator.clipboard && window.isSecureContext) {
          navigator.clipboard.writeText(text).then(showCopyToast).catch(function () {
            if (fallbackCopy(text)) showCopyToast();
          });
        } else if (fallbackCopy(text)) {
          showCopyToast();
        }
      });
    });
  </script>
</body>
</html>`))

func sanitizeFilename(name string) string {
	r := strings.NewReplacer("\"", "", "\r", "", "\n", "", "/", "_", "\\", "_")
	cleaned := strings.TrimSpace(r.Replace(name))
	if cleaned == "" {
		return "subdeck"
	}
	return cleaned
}

func (h *ExportHandler) ExportSubscription(c *gin.Context) {
	token := c.Param("token")
	if token == "" {
		c.String(http.StatusBadRequest, "Missing subscription token")
		return
	}

	profile, err := h.db.GetProfileByToken(token)
	if err != nil || profile == nil {
		c.String(http.StatusNotFound, "Subscription profile not found or token revoked")
		return
	}

	// 1 & 2. Gather and process nodes via shared aggregator service
	nodes, upstreamUserInfo := services.AggregateProfileNodes(h.db, profile)

	// 3. Inspect headers for auto-adaptation
	format := strings.ToLower(c.Query("format"))
	if format == "" {
		format = strings.ToLower(c.Query("target"))
	}
	ua := strings.ToLower(c.GetHeader("User-Agent"))
	accept := strings.ToLower(c.GetHeader("Accept"))

	// Check if request is a regular browser opening the URL directly without format specified
	isBrowser := (strings.Contains(ua, "mozilla") || strings.Contains(ua, "chrome") || strings.Contains(ua, "safari") || strings.Contains(ua, "edge")) &&
		strings.Contains(accept, "text/html") && format == ""

	if isBrowser {
		h.renderLandingPage(c, profile, nodes)
		return
	}

	// 4. Auto-detect format based on client User-Agent
	if format == "" {
		if strings.Contains(ua, "sing-box") || strings.Contains(ua, "singbox") || strings.Contains(ua, "sbox") {
			format = "singbox"
		} else if strings.Contains(ua, "shadowrocket") || strings.Contains(ua, "v2ray") || strings.Contains(ua, "karing") || strings.Contains(ua, "nekobox") || strings.Contains(ua, "quantumult") {
			format = "base64"
		} else {
			format = "clash"
		}
	}

	// 5. Determine full vs provider (Default to FULL configuration out-of-the-box)
	exportType := strings.ToLower(c.Query("type"))
	fullParam := c.Query("full")
	isFull := true
	if exportType == "provider" || exportType == "proxies" || exportType == "outbounds" || fullParam == "0" || fullParam == "false" {
		isFull = false
	} else if exportType == "full" || fullParam == "1" || fullParam == "true" {
		isFull = true
	}

	// 6. Common Subscription Headers
	if upstreamUserInfo != "" {
		c.Header("subscription-userinfo", upstreamUserInfo)
	} else {
		c.Header("subscription-userinfo", "upload=0; download=0; total=1073741824000; expire=0")
	}
	c.Header("profile-update-interval", "24")

	safeName := sanitizeFilename(profile.Name)

	switch format {
	case "singbox", "sing-box", "sbox":
		var data []byte
		var err error
		if isFull {
			data, err = generator.GenerateSingboxFullJSON(nodes, profile.Name)
		} else {
			data, err = generator.GenerateSingboxOutboundsJSON(nodes)
		}
		if err != nil {
			c.String(http.StatusInternalServerError, "Failed to generate Sing-box configuration: "+err.Error())
			return
		}
		c.Header("Content-Type", "application/json; charset=utf-8")
		c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s.json\"", safeName))
		c.Data(http.StatusOK, "application/json; charset=utf-8", data)

	case "base64", "v2ray", "universal":
		content := generator.GenerateBase64(nodes)
		c.Header("Content-Type", "text/plain; charset=utf-8")
		c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s.txt\"", safeName))
		c.String(http.StatusOK, content)

	default: // clash, meta, mihomo
		var data []byte
		var err error
		if isFull {
			data, err = generator.GenerateClashFullYAML(nodes, profile.Name)
		} else {
			data, err = generator.GenerateClashProxiesYAML(nodes)
		}
		if err != nil {
			c.String(http.StatusInternalServerError, "Failed to generate Clash configuration: "+err.Error())
			return
		}
		c.Header("Content-Type", "text/yaml; charset=utf-8")
		c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s.yaml\"", safeName))
		c.Data(http.StatusOK, "text/yaml; charset=utf-8", data)
	}
}

// renderLandingPage displays a clean, responsive web page when opened in a standard web browser
func (h *ExportHandler) renderLandingPage(c *gin.Context, p *models.Profile, nodes []*models.Node) {
	scheme := "http"
	if c.Request.TLS != nil || c.GetHeader("X-Forwarded-Proto") == "https" {
		scheme = "https"
	}
	host := c.Request.Host
	baseURL := fmt.Sprintf("%s://%s/api/sub/%s", scheme, host, p.Token)

	protoCount := make(map[string]int)
	for _, n := range nodes {
		protoCount[string(n.Type)]++
	}
	var protoItems []protoCountItem
	for proto, count := range protoCount {
		protoItems = append(protoItems, protoCountItem{
			Label: strings.ToUpper(proto),
			Count: count,
		})
	}

	data := landingPageData{
		ProfileName:  p.Name,
		NodeCount:    len(nodes),
		ProtoCounts:  protoItems,
		BaseURL:      baseURL,
		ClashFullURL: baseURL + "?format=clash&type=full",
		SingboxURL:   baseURL + "?format=singbox&type=full",
		Base64URL:    baseURL + "?format=base64",
	}

	var buf bytes.Buffer
	if err := landingPageTmpl.Execute(&buf, data); err != nil {
		c.String(http.StatusInternalServerError, "Failed to render landing page")
		return
	}

	c.Data(http.StatusOK, "text/html; charset=utf-8", buf.Bytes())
}
