package app

import (
	"context"

	"github.com/ppxb/miyabi/internal/ent"
	"github.com/ppxb/miyabi/internal/export"
)

// mediaNotifierSet fans one library update out to every media server
// integration (Emby, Plex). All targets are attempted; the first error wins.
type mediaNotifierSet []export.MediaNotifier

func (set mediaNotifierSet) NotifyUpdated(ctx context.Context, localPath string) error {
	var first error
	for _, notifier := range set {
		if err := notifier.NotifyUpdated(ctx, localPath); err != nil && first == nil {
			first = err
		}
	}
	return first
}

func (set mediaNotifierSet) NotifyUpdatedTx(ctx context.Context, tx *ent.Tx, localPath string) error {
	var first error
	for _, notifier := range set {
		if err := notifier.NotifyUpdatedTx(ctx, tx, localPath); err != nil && first == nil {
			first = err
		}
	}
	return first
}
