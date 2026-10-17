package scrape

import (
	"context"
	"fmt"
	"path"
	"strings"

	"github.com/ppxb/miyabi/internal/codeid"
	"github.com/ppxb/miyabi/internal/domain"
	"github.com/ppxb/miyabi/internal/drive"
	"github.com/ppxb/miyabi/internal/ent"
	"github.com/ppxb/miyabi/internal/export"
	mediaimage "github.com/ppxb/miyabi/internal/image"
	"github.com/ppxb/miyabi/internal/nfo"
	"github.com/ppxb/miyabi/internal/pan"
)

// OrganizeInput describes one scraped movie to materialize on 115.
type OrganizeInput struct {
	Source domain.LibrarySource
	Code   string
	Doc    nfo.Movie
	Videos []pan.File
	Poster []byte
	Fanart []byte
}

// OrganizedFile reports where one video ended up on 115.
type OrganizedFile struct {
	ID       string
	ParentID string
	Name     string
	Path     string
}

// OrganizeResult describes the on-115 layout after organization.
type OrganizeResult struct {
	DirID   string
	DirPath string
	Files   []OrganizedFile
	Changed bool
}

// OrganizePanMovie moves a scraped movie's videos into
// <library root>/<prefix>/<code>/ on 115, renames them after the code, and
// uploads the NFO and artwork beside them — the same layout the local STRM
// export produces, but with the real files, so players can mount the drive
// directly. File IDs survive moves, and every step is idempotent: a retry
// finds files already in place and only finishes what is missing.
func OrganizePanMovie(ctx context.Context, sess drive.Session, input OrganizeInput) (OrganizeResult, error) {
	root := input.Source.Directory
	stem := nfo.FileStem(input.Code)
	prefix := nfo.FileStem(codeid.Prefix(input.Code))

	prefixDirID, err := sess.EnsureDir(ctx, root.ID, prefix)
	if err != nil {
		return OrganizeResult{}, fmt.Errorf("ensure 115 prefix directory: %w", err)
	}
	dirID, err := sess.EnsureDir(ctx, prefixDirID, stem)
	if err != nil {
		return OrganizeResult{}, fmt.Errorf("ensure 115 movie directory: %w", err)
	}
	existing, err := listAllFiles(ctx, sess, dirID)
	if err != nil {
		return OrganizeResult{}, err
	}
	present := make(map[string]bool, len(existing))
	for _, entry := range existing {
		present[entry.ID] = true
	}

	videos := export.SortedVideos(input.Videos)
	targets := make(map[string]string, len(videos))
	for i, video := range videos {
		name := stem + path.Ext(video.Name)
		if len(videos) > 1 {
			name = fmt.Sprintf("%s-cd%d%s", stem, i+1, path.Ext(video.Name))
		}
		targets[video.ID] = name
	}

	result := OrganizeResult{
		DirID:   dirID,
		DirPath: strings.TrimSuffix(root.Path, "/") + "/" + prefix + "/" + stem,
	}

	// Move every video that is not in the target directory yet in one batch,
	// then re-list: the listing is the ground truth for the rename pass
	// (115 may suffix a name when a moved file collides).
	var moveIDs []string
	for _, video := range videos {
		if !present[video.ID] {
			moveIDs = append(moveIDs, video.ID)
		}
	}
	if len(moveIDs) > 0 {
		if err := sess.MoveFiles(ctx, dirID, moveIDs); err != nil {
			return OrganizeResult{}, fmt.Errorf("move videos on 115: %w", err)
		}
		result.Changed = true
		if existing, err = listAllFiles(ctx, sess, dirID); err != nil {
			return OrganizeResult{}, err
		}
	}
	occupied := make(map[string]string, len(existing))
	currentName := make(map[string]string, len(existing))
	for _, entry := range existing {
		occupied[entry.Name] = entry.ID
		currentName[entry.ID] = entry.Name
	}

	// Rename videos whose current name differs from the target name. A target
	// name held by a file outside this movie is a hard stop rather than
	// something to delete: it is not ours.
	type renameJob struct{ id, from, to string }
	var pending []renameJob
	own := make(map[string]bool, len(videos))
	for _, video := range videos {
		own[video.ID] = true
	}
	for _, video := range videos {
		if currentName[video.ID] != targets[video.ID] {
			if occupant, taken := occupied[targets[video.ID]]; taken && !own[occupant] {
				return OrganizeResult{}, fmt.Errorf("115 目标目录已有同名文件 %q，无法改名", targets[video.ID])
			}
			pending = append(pending, renameJob{id: video.ID, from: currentName[video.ID], to: targets[video.ID]})
		}
	}
	for len(pending) > 0 {
		var blocked []renameJob
		progressed := false
		for _, job := range pending {
			if occupant, taken := occupied[job.to]; taken && occupant != job.id {
				blocked = append(blocked, job)
				continue
			}
			if err := sess.RenameFile(ctx, job.id, job.to); err != nil {
				return OrganizeResult{}, fmt.Errorf("rename video on 115: %w", err)
			}
			delete(occupied, job.from)
			occupied[job.to] = job.id
			result.Changed = true
			progressed = true
		}
		pending = blocked
		if !progressed && len(pending) > 0 {
			// A name cycle inside this movie's own files: park the first
			// file under a temporary name to break it.
			job := pending[0]
			tmp := stem + ".organizing-" + job.id
			if err := sess.RenameFile(ctx, job.id, tmp); err != nil {
				return OrganizeResult{}, fmt.Errorf("rename video on 115: %w", err)
			}
			delete(occupied, job.from)
			occupied[tmp] = job.id
			pending[0].from = tmp
			result.Changed = true
		}
	}
	for _, video := range videos {
		result.Files = append(result.Files, OrganizedFile{
			ID: video.ID, ParentID: dirID, Name: targets[video.ID],
			Path: result.DirPath + "/" + targets[video.ID],
		})
	}

	// Sidecars mirror WriteMedia: the NFO references poster.jpg/fanart.jpg
	// unconditionally, so players see the same document as the export tree.
	doc := input.Doc
	doc.Thumbs = []nfo.Thumb{{Aspect: "poster", Path: "poster.jpg"}}
	doc.Fanart = "fanart.jpg"
	nfoBody, err := nfo.Encode(doc)
	if err != nil {
		return OrganizeResult{}, fmt.Errorf("encode nfo: %w", err)
	}
	sidecars := []struct {
		name string
		body []byte
	}{{stem + ".nfo", nfoBody}}
	if len(input.Poster) > 0 {
		sidecars = append(sidecars, struct {
			name string
			body []byte
		}{"poster.jpg", input.Poster})
	}
	if len(input.Fanart) > 0 {
		sidecars = append(sidecars, struct {
			name string
			body []byte
		}{"fanart.jpg", input.Fanart})
	}
	for _, sidecar := range sidecars {
		if current, ok := findByName(existing, sidecar.name); ok {
			if current.Size == int64(len(sidecar.body)) {
				continue
			}
			if err := sess.DeleteFiles(ctx, dirID, []string{current.ID}); err != nil {
				return OrganizeResult{}, fmt.Errorf("replace 115 sidecar %s: %w", sidecar.name, err)
			}
		}
		if err := sess.UploadFile(ctx, dirID, sidecar.name, sidecar.body); err != nil {
			return OrganizeResult{}, fmt.Errorf("upload 115 sidecar %s: %w", sidecar.name, err)
		}
		result.Changed = true
	}
	return result, nil
}

// OrganizeLocalMovie organizes an already-scraped movie on 115 from its
// saved metadata and cached artwork, mirroring ExportLocalMovie.
func OrganizeLocalMovie(ctx context.Context, db *ent.Client, sess drive.Session, source domain.LibrarySource, record *ent.Movie, images *mediaimage.Cache) (bool, error) {
	if record.Code == "" || sess == nil {
		return false, nil
	}
	doc, err := MovieNFO(record)
	if err != nil {
		return false, err
	}
	videos := make([]pan.File, 0, len(record.Edges.Files))
	for _, f := range record.Edges.Files {
		videos = append(videos, pan.File{ID: f.FileID, ParentID: f.ParentID, Name: f.Name, Size: f.Size, PickCode: f.PickCode})
	}
	var posterBytes, fanartBytes []byte
	artwork := MovieArtwork(record)
	if artwork.Poster != "" {
		posterBytes, err = images.ReadURL(artwork.Poster)
		if err != nil {
			return false, err
		}
	}
	if artwork.Fanart != "" {
		fanartBytes, err = images.ReadURL(artwork.Fanart)
	} else if artwork.Thumbnail != "" {
		fanartBytes, err = images.ReadURL(artwork.Thumbnail)
	}
	if err != nil {
		return false, err
	}
	result, err := OrganizePanMovie(ctx, sess, OrganizeInput{
		Source: source, Code: record.Code, Doc: doc, Videos: videos, Poster: posterBytes, Fanart: fanartBytes,
	})
	if err != nil {
		return false, err
	}
	placed := make(map[string]OrganizedFile, len(result.Files))
	for _, file := range result.Files {
		placed[file.ID] = file
	}
	organized := make([]pan.File, 0, len(record.Edges.Files))
	for _, entry := range record.Edges.Files {
		target, ok := placed[entry.FileID]
		if !ok {
			continue
		}
		organized = append(organized, pan.File{
			ID: entry.FileID, ParentID: target.ParentID, Name: target.Name,
			SHA1: entry.Sha1, Size: entry.Size,
		})
	}
	err = ent.WithTx(ctx, db, func(tx *ent.Tx) error {
		for _, entry := range record.Edges.Files {
			target, ok := placed[entry.FileID]
			if !ok || (entry.ParentID == target.ParentID && entry.Name == target.Name && entry.Path == target.Path) {
				continue
			}
			if err := tx.File.UpdateOneID(entry.ID).
				SetParentID(target.ParentID).SetName(target.Name).SetPath(target.Path).Exec(ctx); err != nil {
				return err
			}
		}
		// Keep the metadata snapshot describing the organized state, like
		// publish does, so later scans still recognize the movie as current.
		if record.MetadataSnapshot != nil && len(organized) > 0 {
			snapshot := *record.MetadataSnapshot
			snapshot.Videos = VideoFingerprint(organized)
			if err := tx.Movie.UpdateOneID(record.ID).SetMetadataSnapshot(&snapshot).Exec(ctx); err != nil {
				return err
			}
		}
		return nil
	})
	return result.Changed, err
}

func listAllFiles(ctx context.Context, sess drive.Session, dirID string) ([]pan.File, error) {
	var files []pan.File
	for offset := 0; ; {
		page, err := sess.List(ctx, dirID, offset)
		if err != nil {
			return nil, fmt.Errorf("list 115 target directory: %w", err)
		}
		files = append(files, page.Files...)
		offset += len(page.Files)
		if !page.HasMore || len(page.Files) == 0 {
			return files, nil
		}
	}
}

func findByName(files []pan.File, name string) (pan.File, bool) {
	for _, file := range files {
		if file.Name == name {
			return file, true
		}
	}
	return pan.File{}, false
}
