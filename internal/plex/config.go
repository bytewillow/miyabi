package plex

import (
	"net/url"
	"strings"

	"github.com/ppxb/miyabi/internal/domain"
)

const SettingKey = "plex.config"

// Config represents Plex integration settings.
type Config struct {
	Enabled   bool   `json:"enabled"`
	ServerURL string `json:"server_url"`
	Token     string `json:"token"`
	MediaPath string `json:"media_path"`
	LocalDir  string `json:"local_dir"`
}

// Normalize trims inputs and validates required fields when enabled.
func (c *Config) Normalize() error {
	c.ServerURL = strings.TrimRight(strings.TrimSpace(c.ServerURL), "/")
	c.Token = strings.TrimSpace(c.Token)
	c.MediaPath = strings.TrimSpace(c.MediaPath)
	c.LocalDir = strings.TrimSpace(c.LocalDir)

	if c.Enabled {
		if c.ServerURL == "" {
			return domain.E(domain.KindInvalid, "启用 Plex 时必须填写服务器地址", nil)
		}
		u, err := url.Parse(c.ServerURL)
		if err != nil || u.Scheme == "" || u.Host == "" {
			return domain.E(domain.KindInvalid, "Plex 服务器地址格式不正确，需包含 http:// 或 https://", nil)
		}
		if c.Token == "" {
			return domain.E(domain.KindInvalid, "启用 Plex 时必须填写 X-Plex-Token", nil)
		}
	}
	return nil
}

func (c Config) ready() bool { return c.Enabled && c.ServerURL != "" && c.Token != "" }
