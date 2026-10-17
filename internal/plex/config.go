package plex

import (
	"net"
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

	// CloudDrive2 cache refresh (optional): when Plex reads the media
	// library through a CloudDrive2 mount, folders created via the 115
	// API stay invisible until CloudDrive2's directory cache is forced
	// to refresh. When an endpoint is configured, each notification
	// first force-refreshes the CloudDrive2 directories leading to the
	// notified directory, then asks Plex to scan.
	CloudDrive2Endpoint string `json:"clouddrive2_endpoint"`
	CloudDrive2Token    string `json:"clouddrive2_token"`
	// CloudDrive2Path is the CloudDrive2-side path of the directory
	// MediaPath points at, e.g. "/PLEX/AV" when the CloudDrive2 mount
	// root corresponds to the 115 root.
	CloudDrive2Path string `json:"clouddrive2_path"`
}

// Normalize trims inputs and validates required fields when enabled.
func (c *Config) Normalize() error {
	c.ServerURL = strings.TrimRight(strings.TrimSpace(c.ServerURL), "/")
	c.Token = strings.TrimSpace(c.Token)
	c.MediaPath = strings.TrimSpace(c.MediaPath)
	c.LocalDir = strings.TrimSpace(c.LocalDir)
	c.CloudDrive2Endpoint = strings.TrimSpace(c.CloudDrive2Endpoint)
	c.CloudDrive2Token = strings.TrimSpace(c.CloudDrive2Token)
	c.CloudDrive2Path = strings.TrimSpace(c.CloudDrive2Path)

	if c.CloudDrive2Endpoint != "" {
		if _, _, err := net.SplitHostPort(c.CloudDrive2Endpoint); err != nil {
			return domain.E(domain.KindInvalid, "CloudDrive2 地址格式不正确，需为 host:port", nil)
		}
		if c.CloudDrive2Path == "" {
			return domain.E(domain.KindInvalid, "填写 CloudDrive2 地址时必须填写其媒体库路径", nil)
		}
	}

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

func (c Config) cloudDrive2Ready() bool {
	return c.CloudDrive2Endpoint != "" && c.CloudDrive2Path != ""
}
