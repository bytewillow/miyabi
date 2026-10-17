package plex

import (
	"context"
	"log/slog"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/ppxb/miyabi/internal/clouddrive"
)

// cloudDriveClient returns the cached CloudDrive2 client for the current
// configuration, rebuilding it when the endpoint or token changes.
func (s *Service) cloudDriveClient(cfg Config) *clouddrive.Client {
	if !cfg.cloudDrive2Ready() {
		return nil
	}
	key := cfg.CloudDrive2Endpoint + "\x00" + cfg.CloudDrive2Token
	s.cdMu.Lock()
	defer s.cdMu.Unlock()
	if s.cdClient == nil || s.cdKey != key {
		s.cdClient = clouddrive.New(cfg.CloudDrive2Endpoint, cfg.CloudDrive2Token)
		s.cdKey = key
	}
	return s.cdClient
}

// refreshCloudDriveCache force-refreshes the CloudDrive2 directory cache
// for every level leading to plexPath, so the directory is visible through
// the mount when Plex scans it. Failures are logged only: the Plex scan
// still runs, matching how a missing CloudDrive2 behaves today.
func (s *Service) refreshCloudDriveCache(ctx context.Context, cfg Config, plexPath string) {
	cli := s.cloudDriveClient(cfg)
	if cli == nil {
		return
	}
	for _, dir := range cloudDriveRefreshPaths(cfg, plexPath) {
		rctx, cancel := context.WithTimeout(ctx, 30*time.Second)
		err := cli.RefreshDir(rctx, dir)
		cancel()
		if err != nil {
			slog.WarnContext(ctx, "refresh CloudDrive2 directory cache failed", "path", dir, "error", err)
		}
	}
}

// cloudDriveRefreshPaths maps a Plex-view directory to the CloudDrive2
// directories whose cached listings gate its visibility: the configured
// CloudDrive2 root (the counterpart of MediaPath) and each level below it
// down to the directory itself. A parent listing is what makes a child
// directory appear in the mount, so every level needs the refresh.
func cloudDriveRefreshPaths(cfg Config, plexPath string) []string {
	if !cfg.cloudDrive2Ready() || cfg.MediaPath == "" {
		return nil
	}
	mediaPath := strings.TrimRight(filepath.ToSlash(cfg.MediaPath), "/")
	plexPath = path.Clean("/" + filepath.ToSlash(plexPath))
	if plexPath != mediaPath && !strings.HasPrefix(plexPath, mediaPath+"/") {
		return nil
	}
	root := clouddrive.NormalizePath(cfg.CloudDrive2Path)
	dirs := []string{root}
	rel := strings.TrimPrefix(plexPath, mediaPath)
	current := root
	for _, seg := range strings.Split(strings.Trim(rel, "/"), "/") {
		if seg == "" {
			continue
		}
		current = path.Join(current, seg)
		dirs = append(dirs, current)
	}
	return dirs
}
