package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

const DefaultConfigFile = "config.yaml"

// Version is set at build time via -ldflags "-X subdeck/internal/config.Version=..."
var Version = "v0.1.0"

type Config struct {
	Server   ServerConfig   `yaml:"server"`
	Database DatabaseConfig `yaml:"database"`
	Auth     AuthConfig     `yaml:"auth"`
}

type ServerConfig struct {
	Host           string   `yaml:"host"`
	Port           int      `yaml:"port"`
	TrustedProxies []string `yaml:"trusted_proxies"`
	CorsOrigin     string   `yaml:"cors_origin"`
}

type DatabaseConfig struct {
	Path string `yaml:"path"`
}

type AuthConfig struct {
	AdminUsername string `yaml:"admin_username"`
	AdminPassword string `yaml:"admin_password"`
	JWTSecret     string `yaml:"jwt_secret"`
}

// Default returns a Config populated with sensible default values.
func Default() *Config {
	return &Config{
		Server: ServerConfig{
			Host:       "0.0.0.0",
			Port:       8080,
			CorsOrigin: "*",
		},
		Database: DatabaseConfig{
			Path: "data/subs.db",
		},
		Auth: AuthConfig{
			AdminUsername: "admin",
		},
	}
}

// Load reads configuration from YAML file (if present or explicitly requested)
// and applies environment variable overrides.
func Load(path string, explicit bool) (*Config, error) {
	cfg := Default()

	if path == "" {
		path = DefaultConfigFile
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("failed to read config file %s: %w", path, err)
		}
		if explicit {
			return nil, fmt.Errorf("config file not found: %s", path)
		}
	} else {
		if err := yaml.Unmarshal(data, cfg); err != nil {
			return nil, fmt.Errorf("failed to parse config file %s: %w", path, err)
		}
	}

	applyEnvOverrides(cfg)
	normalize(cfg)
	return cfg, nil
}

func getEnvWithFallback(primary, fallback string) string {
	if v := os.Getenv(primary); v != "" {
		return v
	}
	return os.Getenv(fallback)
}

func applyEnvOverrides(cfg *Config) {
	if host := getEnvWithFallback("SUBDECK_HOST", "SUBLITE_HOST"); host != "" {
		cfg.Server.Host = host
	}
	if portStr := getEnvWithFallback("SUBDECK_PORT", "SUBLITE_PORT"); portStr != "" {
		if p, err := strconv.Atoi(portStr); err == nil && p > 0 && p <= 65535 {
			cfg.Server.Port = p
		}
	}
	if proxies := getEnvWithFallback("SUBDECK_TRUSTED_PROXIES", "SUBLITE_TRUSTED_PROXIES"); proxies != "" {
		cfg.Server.TrustedProxies = parseCSV(proxies)
	}
	if cors := getEnvWithFallback("SUBDECK_CORS_ORIGIN", "SUBLITE_CORS_ORIGIN"); cors != "" {
		cfg.Server.CorsOrigin = cors
	}
	if dbPath := getEnvWithFallback("SUBDECK_DB", "SUBLITE_DB"); dbPath != "" {
		cfg.Database.Path = dbPath
	}
	if user := getEnvWithFallback("SUBDECK_ADMIN_USER", "SUBLITE_ADMIN_USER"); user != "" {
		cfg.Auth.AdminUsername = user
	}
	if pass := getEnvWithFallback("SUBDECK_ADMIN_PASS", "SUBLITE_ADMIN_PASS"); pass != "" {
		cfg.Auth.AdminPassword = pass
	}
	if secret := getEnvWithFallback("SUBDECK_JWT_SECRET", "SUBLITE_JWT_SECRET"); secret != "" {
		cfg.Auth.JWTSecret = secret
	}
}

func normalize(cfg *Config) {
	if strings.TrimSpace(cfg.Server.Host) == "" {
		cfg.Server.Host = "0.0.0.0"
	}
	if cfg.Server.Port <= 0 || cfg.Server.Port > 65535 {
		cfg.Server.Port = 8080
	}
	if strings.TrimSpace(cfg.Server.CorsOrigin) == "" {
		cfg.Server.CorsOrigin = "*"
	}
	if strings.TrimSpace(cfg.Database.Path) == "" {
		cfg.Database.Path = "data/subs.db"
	}
	cfg.Auth.AdminUsername = strings.TrimSpace(cfg.Auth.AdminUsername)
	if cfg.Auth.AdminUsername == "" {
		cfg.Auth.AdminUsername = "admin"
	}
}

func parseCSV(raw string) []string {
	var out []string
	for _, part := range strings.Split(raw, ",") {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

// WriteDefault writes an annotated sample config file to the given path.
func WriteDefault(path string) error {
	content := `# SubDeck 配置文件
# 优先级：命令行参数 > 环境变量 (SUBDECK_*) > 本配置文件 > 默认值

server:
  host: "0.0.0.0"
  port: 8080
  # 反代可信 IP/CIDR 列表（如 Nginx/Caddy 本机反代可填 ["127.0.0.1"]）
  trusted_proxies: []
  cors_origin: "*"

database:
  path: "data/subs.db"

auth:
  # 管理员用户名
  admin_username: "admin"
  # 管理员密码（留空则在首次初始化数据库时使用默认密码 admin123；设置后修改此处并重启可同步更新密码）
  admin_password: ""
  # JWT 签名密钥（留空则首次启动自动生成随机密钥并持久化到数据库）
  jwt_secret: ""
`
	return os.WriteFile(path, []byte(content), 0600)
}
