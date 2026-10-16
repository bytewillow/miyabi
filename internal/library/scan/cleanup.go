package scan

import (
	"path"
	"strings"

	"github.com/ppxb/miyabi/internal/domain"
	"github.com/ppxb/miyabi/internal/pan"
)

// CleanupConfig controls scan-time removal of junk files from source folders.
// Deletion uses the 115 recycle bin, so removed files stay recoverable.
type CleanupConfig struct {
	Enabled bool
	// MaxAdVideoSize is the size below which a video counts as an ad clip
	// when the same folder also holds a proper feature video.
	MaxAdVideoSize int64
}

// junkFileExtensions are non-media leftovers that ship inside download
// folders: release-site pages, promo notes and installer stubs. Subtitles,
// images and NFO sidecars are deliberately not junk.
var junkFileExtensions = map[string]bool{
	".html": true, ".htm": true, ".mht": true, ".webloc": true,
	".txt": true, ".url": true, ".lnk": true,
	".exe": true, ".bat": true, ".cmd": true, ".com": true, ".scr": true,
	".apk": true, ".torrent": true,
}

// classifyJunk splits one directory listing into files to delete and the
// videos to keep. A small video is only treated as an ad when the folder
// holds at least one full-size video and the small one does not belong to
// that feature's code group (multi-part releases keep their parts).
func classifyJunk(entries []pan.File, videos []Video, maxAdVideoSize int64) (junk []pan.File, kept []Video) {
	mainCodes := make(map[string]bool)
	hasMain := false
	if maxAdVideoSize > 0 {
		for _, video := range videos {
			if video.Size >= maxAdVideoSize {
				hasMain = true
				if video.Code != "" {
					mainCodes[video.Code] = true
				}
			}
		}
	}
	videoIDs := make(map[string]bool, len(videos))
	for _, video := range videos {
		videoIDs[video.ID] = true
		if hasMain && video.Size < maxAdVideoSize && (video.Code == "" || !mainCodes[video.Code]) {
			junk = append(junk, video.File)
			continue
		}
		kept = append(kept, video)
	}
	for _, entry := range entries {
		if entry.IsDirectory || videoIDs[entry.ID] {
			continue
		}
		if junkFileExtensions[strings.ToLower(path.Ext(entry.Name))] && !domain.IsVideo(entry.Name) {
			junk = append(junk, entry)
		}
	}
	return junk, kept
}
