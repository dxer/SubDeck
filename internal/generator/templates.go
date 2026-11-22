package generator

import (
	_ "embed"
	"fmt"
	"sync"
)

//go:embed templates/clash-default.yaml
var defaultClashTemplateYAML string

//go:embed templates/singbox-default.json
var defaultSingboxTemplateJSON string

var (
	defaultClashOnce   sync.Once
	defaultClashParsed map[string]interface{}
	defaultClashErr    error

	defaultSingboxOnce   sync.Once
	defaultSingboxParsed map[string]interface{}
	defaultSingboxErr    error
)

// DefaultClashTemplate returns a deep copy of the embedded Clash base template
// (static foundation: ports, dns, sniffer, tun, profile). Proxies, groups and
// rules stay generated so rule presets and region groups keep working.
func DefaultClashTemplate() (map[string]interface{}, error) {
	defaultClashOnce.Do(func() {
		m, err := ParseClashTemplate(defaultClashTemplateYAML)
		if err != nil {
			defaultClashErr = fmt.Errorf("built-in clash template invalid: %w", err)
			return
		}
		defaultClashParsed = m
	})
	if defaultClashErr != nil {
		return nil, defaultClashErr
	}
	return deepCopyMap(defaultClashParsed), nil
}

// DefaultSingboxTemplate returns a deep copy of the embedded sing-box base
// template (log, dns servers, inbounds, experimental). Outbounds and route
// stay generated per profile preset.
func DefaultSingboxTemplate() (map[string]interface{}, error) {
	defaultSingboxOnce.Do(func() {
		m, err := ParseSingboxTemplate(defaultSingboxTemplateJSON)
		if err != nil {
			defaultSingboxErr = fmt.Errorf("built-in sing-box template invalid: %w", err)
			return
		}
		defaultSingboxParsed = m
	})
	if defaultSingboxErr != nil {
		return nil, defaultSingboxErr
	}
	return deepCopyMap(defaultSingboxParsed), nil
}
