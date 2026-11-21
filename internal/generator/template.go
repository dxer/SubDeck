package generator

import (
	"encoding/json"
	"fmt"

	"gopkg.in/yaml.v3"
)

// ParseClashTemplate parses a user-supplied YAML template into a generic map.
// The root must be a mapping; comments/formatting are not preserved.
func ParseClashTemplate(text string) (map[string]interface{}, error) {
	var m map[string]interface{}
	if err := yaml.Unmarshal([]byte(text), &m); err != nil {
		return nil, fmt.Errorf("模板 YAML 解析失败: %w", err)
	}
	if m == nil {
		return nil, fmt.Errorf("模板根节点必须是映射对象")
	}
	return m, nil
}

// ParseSingboxTemplate parses a user-supplied JSON template into a generic map.
func ParseSingboxTemplate(text string) (map[string]interface{}, error) {
	var m map[string]interface{}
	if err := json.Unmarshal([]byte(text), &m); err != nil {
		return nil, fmt.Errorf("模板 JSON 解析失败: %w", err)
	}
	if m == nil {
		return nil, fmt.Errorf("模板根节点必须是 JSON 对象")
	}
	return m, nil
}

func deepCopyMap(src map[string]interface{}) map[string]interface{} {
	dst := make(map[string]interface{}, len(src))
	for k, v := range src {
		switch typed := v.(type) {
		case map[string]interface{}:
			dst[k] = deepCopyMap(typed)
		case []interface{}:
			dst[k] = deepCopySlice(typed)
		default:
			dst[k] = v
		}
	}
	return dst
}

func deepCopySlice(src []interface{}) []interface{} {
	dst := make([]interface{}, len(src))
	for i, v := range src {
		switch typed := v.(type) {
		case map[string]interface{}:
			dst[i] = deepCopyMap(typed)
		case []interface{}:
			dst[i] = deepCopySlice(typed)
		default:
			dst[i] = v
		}
	}
	return dst
}

// deepMergeOverlay merges template into base recursively:
//   - nested maps merge recursively
//   - scalars and arrays replace base values
//   - a JSON/YAML null in the template deletes the key from base
func deepMergeOverlay(base, tmpl map[string]interface{}) {
	for k, v := range tmpl {
		if v == nil {
			delete(base, k)
			continue
		}
		if tv, ok := v.(map[string]interface{}); ok {
			if bv, ok := base[k].(map[string]interface{}); ok {
				deepMergeOverlay(bv, tv)
				continue
			}
			base[k] = deepCopyMap(tv)
			continue
		}
		base[k] = v
	}
}

// ApplyClashTemplate overlays a parsed YAML template onto a generated full Clash
// config. Nested maps merge recursively, scalars/arrays replace, and null deletes.
// The generated `proxies` array is protected: template proxy entries are appended
// after the generated nodes and any name collision is rejected so node data can
// never be silently dropped or shadowed.
func ApplyClashTemplate(cfg map[string]interface{}, tmpl map[string]interface{}) error {
	if cfg == nil || tmpl == nil {
		return nil
	}

	overlay := tmpl
	if raw, present := tmpl["proxies"]; present {
		list, ok := raw.([]interface{})
		if !ok {
			return fmt.Errorf("模板 proxies 必须是数组")
		}
		_ = list // validated below against generated names
	}

	// Collect generated node names for collision checking.
	generatedNames := make(map[string]bool)
	if raw, ok := cfg["proxies"].([]interface{}); ok {
		for _, item := range raw {
			if m, ok := item.(map[string]interface{}); ok {
				if name := fmt.Sprint(m["name"]); name != "" && name != "<nil>" {
					generatedNames[name] = true
				}
			}
		}
	}

	if raw, present := tmpl["proxies"]; present {
		list, _ := raw.([]interface{})
		var templateProxies []interface{}
		for _, item := range list {
			m, ok := item.(map[string]interface{})
			if !ok {
				return fmt.Errorf("模板 proxies 每一项必须是映射对象")
			}
			name := fmt.Sprint(m["name"])
			if name != "" && name != "<nil>" && generatedNames[name] {
				return fmt.Errorf("模板节点名与生成节点冲突: %s", name)
			}
			templateProxies = append(templateProxies, deepCopyMap(m))
		}
		merged := make([]interface{}, 0)
		if existing, ok := cfg["proxies"].([]interface{}); ok {
			merged = append(merged, existing...)
		}
		merged = append(merged, templateProxies...)
		cfg["proxies"] = merged

		// Remove proxies from the overlay so the recursive merge never replaces them.
		overlay = deepCopyMap(tmpl)
		delete(overlay, "proxies")
	}

	deepMergeOverlay(cfg, overlay)
	return nil
}

// ApplySingboxTemplate overlays a parsed JSON template onto a generated full
// sing-box config. The generated `outbounds` array (nodes plus control groups)
// is protected: template outbounds are appended and tag collisions are rejected.
func ApplySingboxTemplate(cfg map[string]interface{}, tmpl map[string]interface{}) error {
	if cfg == nil || tmpl == nil {
		return nil
	}

	overlay := tmpl

	// Collect generated outbound tags.
	generatedTags := make(map[string]bool)
	if raw, ok := cfg["outbounds"].([]interface{}); ok {
		for _, item := range raw {
			if m, ok := item.(map[string]interface{}); ok {
				if tag := fmt.Sprint(m["tag"]); tag != "" && tag != "<nil>" {
					generatedTags[tag] = true
				}
			}
		}
	}

	if raw, present := tmpl["outbounds"]; present {
		list, ok := raw.([]interface{})
		if !ok {
			return fmt.Errorf("模板 outbounds 必须是数组")
		}
		seenTemplate := make(map[string]bool)
		var templateOutbounds []interface{}
		for _, item := range list {
			m, ok := item.(map[string]interface{})
			if !ok {
				return fmt.Errorf("模板 outbounds 每一项必须是映射对象")
			}
			tag := fmt.Sprint(m["tag"])
			if tag != "" && tag != "<nil>" {
				if generatedTags[tag] {
					return fmt.Errorf("模板 outbound tag 与生成配置冲突: %s", tag)
				}
				if seenTemplate[tag] {
					return fmt.Errorf("模板 outbound tag 重复: %s", tag)
				}
				seenTemplate[tag] = true
			}
			templateOutbounds = append(templateOutbounds, deepCopyMap(m))
		}
		merged := make([]interface{}, 0)
		if existing, ok := cfg["outbounds"].([]interface{}); ok {
			merged = append(merged, existing...)
		}
		merged = append(merged, templateOutbounds...)
		cfg["outbounds"] = merged

		overlay = deepCopyMap(tmpl)
		delete(overlay, "outbounds")
	}

	deepMergeOverlay(cfg, overlay)
	return nil
}
