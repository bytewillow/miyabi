package organize

import (
	"context"
	"fmt"
	"sync"

	"github.com/ppxb/miyabi/internal/database"
	"github.com/ppxb/miyabi/internal/ent"
)

// Service persists the organization mode and serves the current value to
// the scrape and scan pipelines.
type Service struct {
	db *ent.Client

	mu  sync.RWMutex
	cfg Config
}

func NewService(db *ent.Client) *Service {
	s := &Service{db: db, cfg: Config{Mode: ModeExport}}
	if db != nil {
		if cfg, found, err := database.LoadSetting[Config](context.Background(), db, SettingKey); err == nil && found {
			if err := cfg.Normalize(); err == nil {
				s.cfg = cfg
			}
		}
	}
	return s
}

// Config returns the persisted mode.
func (s *Service) Config(context.Context) (Config, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.cfg, nil
}

// UpdateConfig validates and persists a new mode. It takes effect on the
// next publish; already-organized movies are not migrated either way.
func (s *Service) UpdateConfig(ctx context.Context, cfg Config) (Config, error) {
	if err := cfg.Normalize(); err != nil {
		return Config{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := database.SaveSetting(ctx, s.db, SettingKey, cfg); err != nil {
		return Config{}, fmt.Errorf("save organize setting: %w", err)
	}
	s.cfg = cfg
	return s.cfg, nil
}

// PanEnabled reports whether in-place 115 organization is active.
func (s *Service) PanEnabled() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.cfg.Pan()
}
