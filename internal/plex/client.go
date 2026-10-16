package plex

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/ppxb/miyabi/internal/domain"
)

// plexClient owns HTTP details, while callers supply a configuration snapshot.
type plexClient struct{ http *http.Client }

func newPlexClient() *plexClient { return &plexClient{http: &http.Client{Timeout: 10 * time.Second}} }

func (c *plexClient) request(ctx context.Context, cfg Config, endpoint string, result any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(cfg.ServerURL, "/")+endpoint, nil)
	if err != nil {
		return domain.E(domain.KindInvalid, "创建 Plex 请求失败", err)
	}
	req.Header.Set("X-Plex-Token", cfg.Token)
	req.Header.Set("Accept", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return domain.E(domain.KindUpstream, fmt.Sprintf("无法连接到 Plex 服务器: %v", err), err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return domain.E(domain.KindUnauthorized, "Plex Token 无效或权限不足", nil)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		message, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return domain.E(domain.KindUpstream, fmt.Sprintf("Plex 服务器返回错误 (HTTP %d): %s", resp.StatusCode, message), nil)
	}
	if result != nil {
		if err := json.NewDecoder(resp.Body).Decode(result); err != nil {
			return domain.E(domain.KindUpstream, "解析 Plex 响应失败", err)
		}
	}
	return nil
}

// ServerInfo is the application's response format, independent of Plex's wire format.
type ServerInfo struct {
	ServerName string `json:"server_name"`
	Version    string `json:"version"`
	ID         string `json:"id"`
}

func (c *plexClient) ping(ctx context.Context, cfg Config) (ServerInfo, error) {
	var raw struct {
		MediaContainer struct {
			FriendlyName      string `json:"friendlyName"`
			Version           string `json:"version"`
			MachineIdentifier string `json:"machineIdentifier"`
		} `json:"MediaContainer"`
	}
	if err := c.request(ctx, cfg, "/", &raw); err != nil {
		return ServerInfo{}, err
	}
	return ServerInfo{
		ServerName: raw.MediaContainer.FriendlyName,
		Version:    raw.MediaContainer.Version,
		ID:         raw.MediaContainer.MachineIdentifier,
	}, nil
}

// Section is one Plex library section with the paths it watches on the server.
type Section struct {
	Key       string
	Title     string
	Locations []string
}

func (c *plexClient) sections(ctx context.Context, cfg Config) ([]Section, error) {
	var raw struct {
		MediaContainer struct {
			Directory []struct {
				Key      string `json:"key"`
				Title    string `json:"title"`
				Location []struct {
					Path string `json:"path"`
				} `json:"Location"`
			} `json:"Directory"`
		} `json:"MediaContainer"`
	}
	if err := c.request(ctx, cfg, "/library/sections", &raw); err != nil {
		return nil, err
	}
	sections := make([]Section, 0, len(raw.MediaContainer.Directory))
	for _, dir := range raw.MediaContainer.Directory {
		section := Section{Key: dir.Key, Title: dir.Title}
		for _, loc := range dir.Location {
			if loc.Path != "" {
				section.Locations = append(section.Locations, loc.Path)
			}
		}
		sections = append(sections, section)
	}
	return sections, nil
}

// refresh asks Plex to rescan one directory inside a section (partial scan).
func (c *plexClient) refresh(ctx context.Context, cfg Config, sectionKey, plexPath string) error {
	endpoint := "/library/sections/" + url.PathEscape(sectionKey) + "/refresh?path=" + url.QueryEscape(plexPath)
	return c.request(ctx, cfg, endpoint, nil)
}

// matchSection returns the section whose location is the longest prefix of plexPath.
func matchSection(sections []Section, plexPath string) (Section, bool) {
	var best Section
	bestLen := -1
	for _, section := range sections {
		for _, loc := range section.Locations {
			loc = strings.TrimRight(loc, "/")
			if plexPath == loc || strings.HasPrefix(plexPath, loc+"/") {
				if len(loc) > bestLen {
					best, bestLen = section, len(loc)
				}
			}
		}
	}
	return best, bestLen >= 0
}
