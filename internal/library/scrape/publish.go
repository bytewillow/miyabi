package scrape

import (
	"context"
	"fmt"
	"strings"

	"github.com/ppxb/miyabi/internal/database"
	"github.com/ppxb/miyabi/internal/domain"
	subtitlemeta "github.com/ppxb/miyabi/internal/domain/subtitle"
	"github.com/ppxb/miyabi/internal/drive"
	"github.com/ppxb/miyabi/internal/ent"
	"github.com/ppxb/miyabi/internal/ent/file"
	"github.com/ppxb/miyabi/internal/ent/movie"
	"github.com/ppxb/miyabi/internal/export"
	"github.com/ppxb/miyabi/internal/nfo"
	"github.com/ppxb/miyabi/internal/pan"
	"github.com/ppxb/miyabi/internal/tasks"
)

func (service *Service) publishMovie(ctx context.Context, sess drive.Session, job tasks.Job, input Payload) (*SubtitleTask, error) {
	// The unfinished task retains these cache files during remote queries
	// and local export, until the final transaction publishes movie references.
	artwork := *input.Artwork
	poster, err := service.images.ReadURL(artwork.Poster)
	if err != nil {
		return nil, fmt.Errorf("read cached poster: %w", err)
	}
	fanart, err := service.images.ReadURL(artwork.Fanart)
	if err != nil {
		return nil, fmt.Errorf("read cached fanart: %w", err)
	}
	// Verify current video locations before exporting local files.
	directories, err := service.Directories(ctx, sess, input.MetadataPayload)
	if err != nil {
		return nil, err
	}
	snapshot := &domain.MetadataSnapshot{
		Code:          input.Code,
		AccountID:     input.Source.AccountID,
		DirectoryID:   input.Source.Directory.ID,
		PosterVersion: input.PosterVersion,
	}
	var videos []pan.File
	for i, directory := range directories {
		if err := service.verifyVideoPositions(ctx, sess, directory); err != nil {
			return nil, err
		}
		for _, entry := range directory.Files {
			if directory.VideoIDs[entry.ID] {
				videos = append(videos, entry)
			}
		}
		if err := service.db.Task.UpdateOneID(job.ID).SetProgress((i + 1) * 100 / len(directories)).Exec(ctx); err != nil {
			return nil, err
		}
	}
	// Deduplicate and sort all videos across all directories
	seenVideos := make(map[string]bool, len(videos))
	uniqueVideos := make([]pan.File, 0, len(videos))
	for _, v := range videos {
		if !seenVideos[v.ID] {
			seenVideos[v.ID] = true
			uniqueVideos = append(uniqueVideos, v)
		}
	}
	videos = uniqueVideos
	snapshot.Videos = VideoFingerprint(videos)

	input.Completed = true
	encoded, err := tasks.EncodePayload(input)
	if err != nil {
		return nil, err
	}
	if err := service.exportMgr.WithConfig(func(cfg export.Config) error {
		return sess.WithSource(ctx, func() error {
			record, err := service.db.Movie.Query().Where(movie.IDEQ(input.MovieID)).
				Select(movie.FieldID, movie.FieldManualCode, movie.FieldMetadataSnapshot).Only(ctx)
			if err != nil {
				return err
			}
			if record.ManualCode != input.ManualCode {
				return domain.E(domain.KindConflict, "影片番号已纠正，请使用新的刮削任务", nil)
			}
			// A scan may have reconciled the index while artwork was downloading.
			// Check again under the source/export locks before creating any sidecars.
			files, err := service.db.File.Query().Where(database.LibraryFiles(input.Source), file.MovieIDEQ(input.MovieID)).All(ctx)
			if err != nil {
				return err
			}
			current := make([]pan.File, 0, len(files))
			for _, entry := range files {
				current = append(current, pan.File{ID: entry.FileID, ParentID: entry.ParentID, Name: entry.Name, Size: entry.Size, SHA1: entry.Sha1})
			}
			if len(current) == 0 {
				return domain.E(domain.KindConflict, "媒体文件索引已变化，请重新扫描", nil)
			}
			if VideoFingerprint(current) != snapshot.Videos {
				// A shared task may overlap a rescan that adds or moves a part.
				// Retry publication from saved artwork using fresh directory listings.
				service.dirMu.Lock()
				clear(service.dirCache)
				service.dirMu.Unlock()
				return &domain.RetryError{Cause: domain.E(domain.KindConflict, "媒体文件索引已更新，等待重新导出", nil)}
			}
			var oldDir string
			var placed map[string]OrganizedFile
			if service.panOrganize() {
				result, err := OrganizePanMovie(ctx, sess, OrganizeInput{
					Source: input.Source, Code: input.Code, Doc: input.Document,
					Videos: videos, Poster: poster, Fanart: fanart,
				})
				if err != nil {
					return err
				}
				placed = make(map[string]OrganizedFile, len(result.Files))
				for _, file := range result.Files {
					placed[file.ID] = file
				}
				// The snapshot must describe the organized state, or the
				// next scan would see the moved files as changed content
				// and scrape the movie again.
				organized := make([]pan.File, 0, len(videos))
				for _, video := range videos {
					target := placed[video.ID]
					organized = append(organized, pan.File{
						ID: video.ID, ParentID: target.ParentID, Name: target.Name,
						SHA1: video.SHA1, Size: video.Size,
					})
				}
				snapshot.Videos = VideoFingerprint(organized)
				oldDir = service.previousDir(ctx, cfg.EmbyDir, record, input.Code)
			} else {
				if _, err := export.WriteMedia(cfg, input.Code, input.Document, videos, poster, fanart); err != nil {
					return err
				}
				var err error
				oldDir, err = service.removePreviousExport(ctx, cfg.EmbyDir, record, input.Code)
				if err != nil {
					return err
				}
			}
			return ent.WithTx(ctx, service.db, func(tx *ent.Tx) error {
				update := tx.Movie.UpdateOneID(input.MovieID).SetCode(input.Code).SetMetadata(&input.Document).
					SetCover(artwork.Thumbnail).SetPoster(artwork.Poster).SetFanarts([]string{artwork.Fanart}).
					SetScrapeStatus(movie.ScrapeStatusDone).SetMetadataSnapshot(snapshot)
				if id := input.Document.JavDBID(); id != "" {
					update.SetJavdbID(id)
				}
				if err := update.Exec(ctx); err != nil {
					return err
				}
				if err := tx.Task.UpdateOneID(job.ID).SetPayload(encoded).Exec(ctx); err != nil {
					return err
				}
				for _, entry := range files {
					target, ok := placed[entry.FileID]
					if !ok || (entry.ParentID == target.ParentID && entry.Name == target.Name && entry.Path == target.Path) {
						continue
					}
					if err := tx.File.UpdateOneID(entry.ID).
						SetParentID(target.ParentID).SetName(target.Name).SetPath(target.Path).Exec(ctx); err != nil {
						return err
					}
				}
				if service.mediaNotifier != nil && cfg.EmbyDir != "" {
					if oldDir != "" {
						if err := service.mediaNotifier.NotifyUpdatedTx(ctx, tx, oldDir); err != nil {
							return err
						}
					}
					return service.mediaNotifier.NotifyUpdatedTx(ctx, tx, export.EmbyMovieDir(cfg.EmbyDir, input.Code))
				}
				return nil
			})
		})
	}); err != nil {
		return nil, fmt.Errorf("publish movie: %w", err)
	}

	// Subtitle downloads target the exported .strm; in-place organization
	// has no local file to sit beside, so it skips the subtitle follow-up.
	var subTask *SubtitleTask
	if !service.panOrganize() {
		subTask = service.subtitleTask(input.MetadataPayload, videos)
	}

	if service.notifier != nil {
		service.notifier.NotifyLibraryChanged()
	}
	return subTask, nil
}

// previousDir returns the notification path of the movie's previous code,
// mirroring removePreviousExport's guards without touching the local tree:
// in-place organization leaves nothing behind locally to remove.
func (service *Service) previousDir(ctx context.Context, root string, record *ent.Movie, code string) string {
	if root == "" || record.MetadataSnapshot == nil || record.MetadataSnapshot.Code == "" {
		return ""
	}
	oldCode := record.MetadataSnapshot.Code
	oldDir := export.EmbyMovieDir(root, oldCode)
	if strings.EqualFold(oldDir, export.EmbyMovieDir(root, code)) {
		return ""
	}
	claimed, err := service.db.Movie.Query().Where(movie.CodeEQ(oldCode), movie.IDNEQ(record.ID)).Exist(ctx)
	if err != nil || claimed {
		return ""
	}
	return oldDir
}

// subtitleTask targets the .strm exported for a movie's video. Multi-part
// movies export one .strm per part, and whole-movie subtitles fit none of them.
func (service *Service) subtitleTask(input MetadataPayload, videos []pan.File) *SubtitleTask {
	if service.subtitles == nil || len(videos) != 1 {
		return nil
	}
	return &SubtitleTask{
		MovieID: input.MovieID,
		Target: subtitlemeta.Target{
			Dir:           export.EmbyMovieDir(service.exportConfig().EmbyDir, input.Code),
			Stem:          nfo.FileStem(input.Code),
			Code:          input.Code,
			Uncensored:    subtitlemeta.IsUncensored(videos[0].Name),
			HardSubtitled: subtitlemeta.HasHardSubtitle(videos[0].Name),
		},
	}
}

func (service *Service) verifyVideoPositions(ctx context.Context, sess drive.Session, directory MovieDirectory) error {
	for videoID := range directory.VideoIDs {
		info, err := sess.Info(ctx, videoID)
		if err != nil {
			return fmt.Errorf("确认视频文件位置: %w", err)
		}
		if info.ParentID != directory.ID || !drive.WithinSource(info, sess.Source()) {
			return domain.E(domain.KindConflict, "视频已移动，请重新扫描", nil)
		}
	}
	return nil
}
