package plex

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ppxb/miyabi/internal/database"
	"github.com/ppxb/miyabi/internal/ent"
)

func TestConfigNormalizeRequiresTokenWhenEnabled(t *testing.T) {
	cfg := Config{Enabled: true, ServerURL: "http://plex.local:32400/"}
	if err := cfg.Normalize(); err == nil || !strings.Contains(err.Error(), "Token") {
		t.Fatalf("expected token error, got %v", err)
	}
	cfg.Token = " tok "
	if err := cfg.Normalize(); err != nil {
		t.Fatal(err)
	}
	if cfg.ServerURL != "http://plex.local:32400" || cfg.Token != "tok" {
		t.Fatalf("not normalized: %+v", cfg)
	}
	bad := Config{Enabled: true, ServerURL: "plex.local", Token: "x"}
	if err := bad.Normalize(); err == nil {
		t.Fatal("expected scheme error")
	}
	disabled := Config{}
	if err := disabled.Normalize(); err != nil {
		t.Fatalf("disabled config must pass: %v", err)
	}
}

func TestMatchSectionLongestPrefix(t *testing.T) {
	sections := []Section{
		{Key: "1", Locations: []string{"/media"}},
		{Key: "2", Locations: []string{"/media/movies"}},
	}
	got, ok := matchSection(sections, "/media/movies/A (2024)")
	if !ok || got.Key != "2" {
		t.Fatalf("expected section 2, got %+v ok=%v", got, ok)
	}
	if _, ok := matchSection(sections, "/media2/x"); ok {
		t.Fatal("prefix must respect path boundaries")
	}
	if _, ok := matchSection(sections, "/other"); ok {
		t.Fatal("unrelated path must not match")
	}
}

// Exercise individual worker passes without a timer racing the test.
func notificationService(t *testing.T, db *ent.Client, cfg Config) *Service {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	s := &Service{db: db, cfg: cfg, client: newPlexClient(), ctx: ctx, cancel: cancel, wakeNotifications: make(chan struct{}, 1)}
	t.Cleanup(s.Close)
	return s
}

func TestNotificationsDeliverPartialScanAndBackoffOnFailure(t *testing.T) {
	var healthy atomic.Bool
	var refreshes atomic.Int32
	var lastPath, lastToken atomic.Value
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/library/sections":
			if !healthy.Load() {
				w.WriteHeader(http.StatusServiceUnavailable)
				return
			}
			_, _ = w.Write([]byte(`{"MediaContainer":{"Directory":[{"key":"2","title":"Movies","Location":[{"id":1,"path":"/media"}]}]}}`))
		case r.URL.Path == "/library/sections/2/refresh":
			refreshes.Add(1)
			lastPath.Store(r.URL.Query().Get("path"))
			lastToken.Store(r.Header.Get("X-Plex-Token"))
			w.WriteHeader(http.StatusOK)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	dir := t.TempDir()
	store, err := database.Open(t.Context(), dir)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	cfg := Config{Enabled: true, ServerURL: server.URL, Token: "tok", LocalDir: dir, MediaPath: "/media"}
	s := notificationService(t, store.Client, cfg)

	movieDir := filepath.Join(dir, "A (2024)")
	for _, p := range []string{movieDir, movieDir} {
		if err := s.NotifyUpdated(t.Context(), p); err != nil {
			t.Fatal(err)
		}
	}
	if count := store.Client.PlexNotification.Query().CountX(t.Context()); count != 1 {
		t.Fatalf("deduplicated count = %d", count)
	}

	s.flushNotifications(t.Context())
	row := store.Client.PlexNotification.Query().OnlyX(t.Context())
	if row.Attempts != 1 || row.LastError == "" || time.Until(row.NextAttemptAt) <= 0 {
		t.Fatalf("missing failure/backoff: %+v", row)
	}

	healthy.Store(true)
	if err := s.RetryPending(t.Context()); err != nil {
		t.Fatal(err)
	}
	s.flushNotifications(t.Context())
	if count := store.Client.PlexNotification.Query().CountX(t.Context()); count != 0 {
		t.Fatalf("delivered rows must be removed, got %d", count)
	}
	if refreshes.Load() != 1 || lastPath.Load() != "/media/A (2024)" || lastToken.Load() != "tok" {
		t.Fatalf("refresh=%d path=%v token=%v", refreshes.Load(), lastPath.Load(), lastToken.Load())
	}
}

func TestPingParsesServerInfo(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		_, _ = w.Write([]byte(`{"MediaContainer":{"friendlyName":"homeplex","version":"1.41.0","machineIdentifier":"abc123"}}`))
	}))
	defer server.Close()
	info, err := newPlexClient().ping(t.Context(), Config{ServerURL: server.URL, Token: "tok"})
	if err != nil {
		t.Fatal(err)
	}
	if info.ServerName != "homeplex" || info.Version != "1.41.0" || info.ID != "abc123" {
		t.Fatalf("unexpected info: %+v", info)
	}
}
