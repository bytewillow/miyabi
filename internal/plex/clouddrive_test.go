package plex

import (
	"reflect"
	"testing"
)

func TestCloudDriveRefreshPaths(t *testing.T) {
	cfg := Config{
		MediaPath:           "/CloudNAS/115/PLEX/AV",
		CloudDrive2Endpoint: "192.168.12.222:19798",
		CloudDrive2Path:     "/PLEX/AV",
	}
	got := cloudDriveRefreshPaths(cfg, "/CloudNAS/115/PLEX/AV/MIDA/MIDA-827")
	want := []string{"/PLEX/AV", "/PLEX/AV/MIDA", "/PLEX/AV/MIDA/MIDA-827"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("paths = %v, want %v", got, want)
	}

	// The library root itself refreshes only the root listing.
	got = cloudDriveRefreshPaths(cfg, "/CloudNAS/115/PLEX/AV")
	if !reflect.DeepEqual(got, []string{"/PLEX/AV"}) {
		t.Fatalf("root paths = %v", got)
	}

	// A mount-name prefix on the CloudDrive2 side is preserved.
	cfg.CloudDrive2Path = "/115open/PLEX/AV/"
	got = cloudDriveRefreshPaths(cfg, "/CloudNAS/115/PLEX/AV/FNS/FNS-121")
	want = []string{"/115open/PLEX/AV", "/115open/PLEX/AV/FNS", "/115open/PLEX/AV/FNS/FNS-121"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("prefixed paths = %v, want %v", got, want)
	}

	// Directories outside the media path are not mapped.
	if got := cloudDriveRefreshPaths(cfg, "/CloudNAS/115/PLEX/OTHER/X"); got != nil {
		t.Fatalf("outside paths = %v, want nil", got)
	}

	// Unconfigured CloudDrive2 disables the mapping entirely.
	cfg.CloudDrive2Endpoint = ""
	if got := cloudDriveRefreshPaths(cfg, "/CloudNAS/115/PLEX/AV/MIDA/MIDA-827"); got != nil {
		t.Fatalf("unconfigured paths = %v, want nil", got)
	}
}

func TestConfigNormalizeCloudDrive2(t *testing.T) {
	cfg := Config{CloudDrive2Endpoint: "192.168.12.222"}
	if err := cfg.Normalize(); err == nil {
		t.Fatal("expected host:port error")
	}
	cfg.CloudDrive2Endpoint = "192.168.12.222:19798"
	if err := cfg.Normalize(); err == nil {
		t.Fatal("expected missing path error")
	}
	cfg.CloudDrive2Path = " /PLEX/AV "
	if err := cfg.Normalize(); err != nil {
		t.Fatal(err)
	}
	if cfg.CloudDrive2Path != "/PLEX/AV" {
		t.Fatalf("path not trimmed: %q", cfg.CloudDrive2Path)
	}
}
