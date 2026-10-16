// Package strmurl owns the base URL written into generated .strm files.
// It used to live only in the Emby settings, which forced Plex-only users
// to configure Emby just to fix the playback address; it is a standalone
// setting now, applied to the shared export manager on save.
package strmurl

import (
	"context"
	"fmt"
	"sync"

	"github.com/ppxb/miyabi/internal/database"
	"github.com/ppxb/miyabi/internal/ent"
	"github.com/ppxb/miyabi/internal/export"
)

// Service persists the STRM base URL and applies it to the export manager.
type Service struct {
	db        *ent.Client
	exportMgr *export.Manager
	fallback  string

	mu  sync.RWMutex
	cfg Config
}

func NewService(db *ent.Client, exportMgr *export.Manager, fallback string) *Service {
	s := &Service{db: db, exportMgr: exportMgr, fallback: fallback}
	if db != nil {
		if cfg, found, err := database.LoadSetting[Config](context.Background(), db, SettingKey); err == nil && found {
			s.cfg = cfg
		}
	}
	if s.cfg.PublicURL != "" {
		s.apply(s.cfg.PublicURL)
	}
	return s
}

func (s *Service) effective() string {
	if s.cfg.PublicURL != "" {
		return s.cfg.PublicURL
	}
	return s.fallback
}

func (s *Service) apply(publicURL string) {
	if s.exportMgr == nil {
		return
	}
	cfg := s.exportMgr.Config()
	cfg.PublicURL = publicURL
	s.exportMgr.Set(cfg)
}

// Config returns the saved override and the currently effective URL.
func (s *Service) Config(context.Context) (Response, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return Response{PublicURL: s.cfg.PublicURL, EffectiveURL: s.effective()}, nil
}

// UpdateConfig validates, persists, applies, and rewrites every .strm file
// so existing exports pick up the new address immediately.
func (s *Service) UpdateConfig(ctx context.Context, cfg Config) (Response, error) {
	if err := cfg.Normalize(); err != nil {
		return Response{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := database.SaveSetting(ctx, s.db, SettingKey, cfg); err != nil {
		return Response{}, fmt.Errorf("save strm setting: %w", err)
	}
	s.cfg = cfg
	s.apply(s.effective())
	rewritten := 0
	if s.exportMgr != nil {
		count, err := s.exportMgr.RewriteSTRM(ctx)
		if err != nil {
			return Response{}, fmt.Errorf("rewrite strm files: %w", err)
		}
		rewritten = count
	}
	return Response{PublicURL: s.cfg.PublicURL, EffectiveURL: s.effective(), Rewritten: rewritten}, nil
}
