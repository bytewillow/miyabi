package scrape

import (
	"context"
	"strings"
	"testing"

	"github.com/ppxb/miyabi/internal/domain"
	"github.com/ppxb/miyabi/internal/drive"
	"github.com/ppxb/miyabi/internal/nfo"
	"github.com/ppxb/miyabi/internal/pan"
)

// organizeSession is a stateful in-memory 115 tree for engine tests.
type organizeSession struct {
	drive.Session
	source   domain.LibrarySource
	files    map[string]*pan.File
	children map[string][]string
	seq      int
	uploads  []string
}

func newOrganizeSession() *organizeSession {
	s := &organizeSession{
		source: domain.LibrarySource{
			AccountID: "acct",
			Directory: domain.LibraryDirectory{ID: "root", Name: "AV", Path: "/PLEX/AV"},
		},
		files:    map[string]*pan.File{},
		children: map[string][]string{},
	}
	s.files["root"] = &pan.File{ID: "root", Name: "AV", IsDirectory: true}
	return s
}

func (s *organizeSession) Source() domain.LibrarySource { return s.source }

func (s *organizeSession) add(parentID string, file pan.File) {
	entry := file
	s.files[entry.ID] = &entry
	s.children[parentID] = append(s.children[parentID], entry.ID)
}

func (s *organizeSession) EnsureDir(_ context.Context, parentID, name string) (string, error) {
	for _, id := range s.children[parentID] {
		if entry := s.files[id]; entry.IsDirectory && entry.Name == name {
			return id, nil
		}
	}
	s.seq++
	id := "dir-" + string(rune('a'+s.seq))
	s.add(parentID, pan.File{ID: id, ParentID: parentID, Name: name, IsDirectory: true})
	return id, nil
}

func (s *organizeSession) List(_ context.Context, dirID string, offset int) (pan.FilePage, error) {
	ids := s.children[dirID]
	page := pan.FilePage{Total: len(ids)}
	for i := offset; i < len(ids) && len(page.Files) < 100; i++ {
		page.Files = append(page.Files, *s.files[ids[i]])
	}
	page.HasMore = offset+len(page.Files) < len(ids)
	return page, nil
}

func (s *organizeSession) MoveFiles(_ context.Context, targetDirID string, fileIDs []string) error {
	for _, id := range fileIDs {
		entry := s.files[id]
		old := s.children[entry.ParentID]
		for i, child := range old {
			if child == id {
				s.children[entry.ParentID] = append(old[:i], old[i+1:]...)
				break
			}
		}
		entry.ParentID = targetDirID
		s.children[targetDirID] = append(s.children[targetDirID], id)
	}
	return nil
}

func (s *organizeSession) RenameFile(_ context.Context, fileID, name string) error {
	s.files[fileID].Name = name
	return nil
}

func (s *organizeSession) DeleteFiles(_ context.Context, parentID string, fileIDs []string) error {
	for _, id := range fileIDs {
		delete(s.files, id)
		old := s.children[parentID]
		for i, child := range old {
			if child == id {
				s.children[parentID] = append(old[:i], old[i+1:]...)
				break
			}
		}
	}
	return nil
}

func (s *organizeSession) UploadFile(_ context.Context, dirID, name string, content []byte) error {
	s.seq++
	s.add(dirID, pan.File{ID: "up-" + name, ParentID: dirID, Name: name, Size: int64(len(content))})
	s.uploads = append(s.uploads, name)
	return nil
}

func TestOrganizePanMovie_SingleVideo(t *testing.T) {
	ctx := context.Background()
	sess := newOrganizeSession()
	sess.add("root", pan.File{ID: "v1", ParentID: "root", Name: "abp-984.mp4", Size: 10, SHA1: "ABC"})

	result, err := OrganizePanMovie(ctx, sess, OrganizeInput{
		Source: sess.source,
		Code:   "ABP-984",
		Doc:    nfo.Movie{Code: "ABP-984", Title: "title"},
		Videos: []pan.File{{ID: "v1", ParentID: "root", Name: "abp-984.mp4", Size: 10, SHA1: "ABC"}},
		Poster: []byte("poster-bytes"),
	})
	if err != nil {
		t.Fatalf("OrganizePanMovie: %v", err)
	}
	if result.DirPath != "/PLEX/AV/ABP/ABP-984" {
		t.Fatalf("DirPath = %q", result.DirPath)
	}
	if len(result.Files) != 1 || result.Files[0].Name != "ABP-984.mp4" ||
		result.Files[0].Path != "/PLEX/AV/ABP/ABP-984/ABP-984.mp4" ||
		result.Files[0].ParentID != result.DirID {
		t.Fatalf("Files = %+v", result.Files)
	}
	if sess.files["v1"].ParentID != result.DirID || sess.files["v1"].Name != "ABP-984.mp4" {
		t.Fatalf("video not organized: %+v", sess.files["v1"])
	}
	joined := strings.Join(sess.uploads, ",")
	if !strings.Contains(joined, "ABP-984.nfo") || !strings.Contains(joined, "poster.jpg") {
		t.Fatalf("uploads = %v", sess.uploads)
	}
	if !result.Changed {
		t.Fatal("Changed = false, want true")
	}

	// A second run over the organized state must be a no-op.
	again, err := OrganizePanMovie(ctx, sess, OrganizeInput{
		Source: sess.source,
		Code:   "ABP-984",
		Doc:    nfo.Movie{Code: "ABP-984", Title: "title"},
		Videos: []pan.File{{ID: "v1", ParentID: result.DirID, Name: "ABP-984.mp4", Size: 10, SHA1: "ABC"}},
		Poster: []byte("poster-bytes"),
	})
	if err != nil {
		t.Fatalf("second OrganizePanMovie: %v", err)
	}
	if again.Changed {
		t.Fatalf("second run Changed = true, uploads now %v", sess.uploads)
	}
}

func TestOrganizePanMovie_MultiPartNaming(t *testing.T) {
	ctx := context.Background()
	sess := newOrganizeSession()
	sess.add("root", pan.File{ID: "v2", ParentID: "root", Name: "movie-part2.mkv", Size: 10})
	sess.add("root", pan.File{ID: "v1", ParentID: "root", Name: "movie-part1.mkv", Size: 10})

	result, err := OrganizePanMovie(ctx, sess, OrganizeInput{
		Source: sess.source,
		Code:   "MIDE-001",
		Doc:    nfo.Movie{Code: "MIDE-001", Title: "title"},
		Videos: []pan.File{
			{ID: "v2", ParentID: "root", Name: "movie-part2.mkv", Size: 10},
			{ID: "v1", ParentID: "root", Name: "movie-part1.mkv", Size: 10},
		},
	})
	if err != nil {
		t.Fatalf("OrganizePanMovie: %v", err)
	}
	names := map[string]string{}
	for _, file := range result.Files {
		names[file.ID] = file.Name
	}
	if names["v1"] != "MIDE-001-cd1.mkv" || names["v2"] != "MIDE-001-cd2.mkv" {
		t.Fatalf("part names = %v", names)
	}
}

func TestOrganizePanMovie_ForeignNameCollision(t *testing.T) {
	ctx := context.Background()
	sess := newOrganizeSession()
	sess.add("root", pan.File{ID: "v1", ParentID: "root", Name: "abp-984.mp4", Size: 10})
	prefixID, err := sess.EnsureDir(ctx, "root", "ABP")
	if err != nil {
		t.Fatal(err)
	}
	movieDirID, err := sess.EnsureDir(ctx, prefixID, "ABP-984")
	if err != nil {
		t.Fatal(err)
	}
	sess.add(movieDirID, pan.File{ID: "foreign", ParentID: movieDirID, Name: "ABP-984.mp4", Size: 99})

	_, err = OrganizePanMovie(ctx, sess, OrganizeInput{
		Source: sess.source,
		Code:   "ABP-984",
		Doc:    nfo.Movie{Code: "ABP-984", Title: "title"},
		Videos: []pan.File{{ID: "v1", ParentID: "root", Name: "abp-984.mp4", Size: 10}},
	})
	if err == nil || !strings.Contains(err.Error(), "同名文件") {
		t.Fatalf("err = %v, want a name-collision error", err)
	}
}
