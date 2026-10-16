package strmurl

import (
	"fmt"
	"net/url"
	"strings"
)

// SettingKey is the settings-table key for the persisted STRM base URL.
const SettingKey = "strm.config"

// Config is the persisted STRM playback base-URL override. Empty means the
// application default (MIYABI_PUBLIC_URL, else the auto-detected address).
type Config struct {
	PublicURL string `json:"public_url"`
}

// Response is what the settings API returns: the saved override plus the
// address STRM files currently embed.
type Response struct {
	PublicURL    string `json:"public_url"`
	EffectiveURL string `json:"effective_url"`
	Rewritten    int    `json:"rewritten,omitempty"`
}

func (c *Config) Normalize() error {
	c.PublicURL = strings.TrimRight(strings.TrimSpace(c.PublicURL), "/")
	if c.PublicURL == "" {
		return nil
	}
	parsed, err := url.Parse(c.PublicURL)
	if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return fmt.Errorf("STRM 播放地址必须是完整的 http(s) 地址，例如 http://192.168.1.10:8088")
	}
	return nil
}
