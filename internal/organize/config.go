// Package organize owns the library organization mode: scraped movies are
// either written to the local STRM export tree (the upstream behavior) or
// organized in place on 115, where the real media files are moved into the
// same <prefix>/<code> layout and the NFO and artwork are uploaded beside
// them, so any player can mount the drive directly.
package organize

import "fmt"

// SettingKey is the settings-table key for the persisted mode.
const SettingKey = "organize.config"

// Mode selects how a completed scrape is materialized.
type Mode string

const (
	// ModeExport writes the local STRM/NFO tree under the data directory.
	ModeExport Mode = "export"
	// ModePan moves media files into the layout on 115 and uploads sidecars.
	ModePan Mode = "pan"
)

// Config is the persisted organization mode. The zero value is ModeExport.
type Config struct {
	Mode Mode `json:"mode"`
}

func (c *Config) Normalize() error {
	switch c.Mode {
	case "":
		c.Mode = ModeExport
	case ModeExport, ModePan:
	default:
		return fmt.Errorf("组织方式必须是 export 或 pan")
	}
	return nil
}

// Pan reports whether in-place 115 organization is active.
func (c Config) Pan() bool { return c.Mode == ModePan }
