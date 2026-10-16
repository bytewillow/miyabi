package plex

import (
	"context"
	"fmt"
	"path"
	"path/filepath"
	"strings"
	"sync"

	"github.com/ppxb/miyabi/internal/database"
	"github.com/ppxb/miyabi/internal/domain"
	"github.com/ppxb/miyabi/internal/ent"
)

// Service manages configuration persistence and batched scan notifications to Plex.
type Service struct {
	db                *ent.Client
	mu                sync.RWMutex
	cfg               Config
	client            *plexClient
	wakeNotifications chan struct{}
	ctx               context.Context
	cancel            context.CancelFunc
	wg                sync.WaitGroup
	updateMu          sync.Mutex
}

// NewService restores configuration and starts the notification worker.
func NewService(ctx context.Context, db *ent.Client, initial Config) (*Service, error) {
	loaded, found, err := database.LoadSetting[Config](ctx, db, SettingKey)
	if err != nil {
		return nil, fmt.Errorf("load plex setting: %w", err)
	}

	cfg := initial
	if found {
		cfg = loaded
		if cfg.LocalDir == "" {
			cfg.LocalDir = initial.LocalDir
		}
	}

	subCtx, cancel := context.WithCancel(context.Background())
	s := &Service{
		db:                db,
		cfg:               cfg,
		client:            newPlexClient(),
		wakeNotifications: make(chan struct{}, 1),
		ctx:               subCtx,
		cancel:            cancel,
	}
	s.wg.Add(1)
	go s.worker(subCtx)
	return s, nil
}

// Close makes a final delivery attempt; outstanding updates remain in the database.
func (s *Service) Close() {
	s.cancel()
	s.wg.Wait()
}

// Config returns the current active configuration.
func (s *Service) Config(context.Context) (Config, error) { return s.currentConfig(), nil }

func (s *Service) currentConfig() Config {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.cfg
}

// UpdateConfig validates and persists the new configuration to the database.
func (s *Service) UpdateConfig(ctx context.Context, cfg Config) error {
	s.updateMu.Lock()
	defer s.updateMu.Unlock()
	s.mu.Lock()
	if cfg.LocalDir == "" {
		cfg.LocalDir = s.cfg.LocalDir
	}
	s.mu.Unlock()

	if err := cfg.Normalize(); err != nil {
		return err
	}
	if err := database.SaveSetting(ctx, s.db, SettingKey, cfg); err != nil {
		return fmt.Errorf("save plex setting: %w", err)
	}
	s.mu.Lock()
	s.cfg = cfg
	s.mu.Unlock()
	return s.RetryPending(ctx)
}

// Test validates connection parameters by querying the server root.
func (s *Service) Test(ctx context.Context, cfg Config) (ServerInfo, error) {
	serverURL := strings.TrimRight(strings.TrimSpace(cfg.ServerURL), "/")
	token := strings.TrimSpace(cfg.Token)
	if serverURL == "" || token == "" {
		return ServerInfo{}, domain.E(domain.KindInvalid, "请先填写 Plex 服务器地址与 X-Plex-Token", nil)
	}
	return s.client.ping(ctx, Config{ServerURL: serverURL, Token: token})
}

// translatePath mirrors the Emby integration: map a local export directory to
// the path the media server sees, using LocalDir -> MediaPath when configured.
func translatePath(localPath, localDir, mediaPath string) string {
	localPath = filepath.Clean(localPath)
	if mediaPath == "" {
		if !filepath.IsAbs(localPath) && !strings.HasPrefix(localPath, "/") && !strings.HasPrefix(localPath, `\\`) {
			if abs, err := filepath.Abs(localPath); err == nil {
				localPath = abs
			}
		}
		if filepath.IsAbs(localPath) && filepath.VolumeName(localPath) != "" {
			return filepath.Clean(localPath)
		}
		return filepath.ToSlash(localPath)
	}

	mediaPath = strings.TrimRight(filepath.ToSlash(mediaPath), "/")
	if localDir != "" {
		cleanLocalDir := filepath.Clean(localDir)
		rel, err := filepath.Rel(cleanLocalDir, localPath)
		if err == nil {
			slashRel := filepath.ToSlash(rel)
			if slashRel != ".." && !strings.HasPrefix(slashRel, "../") {
				return path.Join(mediaPath, slashRel)
			}
		}
		if absLocalDir, err1 := filepath.Abs(cleanLocalDir); err1 == nil {
			if absLocalPath, err2 := filepath.Abs(localPath); err2 == nil {
				if rel2, err3 := filepath.Rel(absLocalDir, absLocalPath); err3 == nil {
					slashRel2 := filepath.ToSlash(rel2)
					if slashRel2 != ".." && !strings.HasPrefix(slashRel2, "../") {
						return path.Join(mediaPath, slashRel2)
					}
				}
			}
		}
	}

	return ""
}
