package scan

import (
	"testing"

	"github.com/ppxb/miyabi/internal/pan"
)

const mb = int64(1) << 20

func TestClassifyJunk(t *testing.T) {
	main := Video{File: pan.File{ID: "main", Name: "ABC-123.mp4", Size: 2000 * mb}, Code: "ABC-123"}
	part := Video{File: pan.File{ID: "part", Name: "ABC-123-cd2.mp4", Size: 50 * mb}, Code: "ABC-123"}
	ad := Video{File: pan.File{ID: "ad", Name: "最新地址发布.mp4", Size: 8 * mb}}
	adFakeCode := Video{File: pan.File{ID: "ad2", Name: "XYZ-999.mp4", Size: 12 * mb}, Code: "XYZ-999"}
	entries := []pan.File{
		main.File, part.File, ad.File, adFakeCode.File,
		{ID: "html", Name: "发布页.html", Size: 2},
		{ID: "txt", Name: "说明.txt", Size: 2},
		{ID: "url", Name: "官网.url", Size: 2},
		{ID: "nfo", Name: "ABC-123.nfo", Size: 2},
		{ID: "srt", Name: "ABC-123.zh-CN.srt", Size: 2},
		{ID: "jpg", Name: "poster.jpg", Size: 2},
		{ID: "dir", Name: "sub", IsDirectory: true},
	}
	junk, kept := classifyJunk(entries, []Video{main, part, ad, adFakeCode}, 100*mb)
	gotJunk := make(map[string]bool)
	for _, f := range junk {
		gotJunk[f.ID] = true
	}
	for _, id := range []string{"ad", "ad2", "html", "txt", "url"} {
		if !gotJunk[id] {
			t.Errorf("expected %s in junk, junk=%v", id, gotJunk)
		}
	}
	for _, id := range []string{"main", "part", "nfo", "srt", "jpg", "dir"} {
		if gotJunk[id] {
			t.Errorf("did not expect %s in junk", id)
		}
	}
	if len(kept) != 2 || kept[0].ID != "main" || kept[1].ID != "part" {
		t.Fatalf("kept = %+v", kept)
	}
}

func TestClassifyJunkKeepsLoneSmallVideo(t *testing.T) {
	// A folder whose only video is small (e.g. an FC2 clip) has no full-size
	// feature, so nothing video-like may be deleted.
	small := Video{File: pan.File{ID: "small", Name: "FC2-PPV-123.mp4", Size: 60 * mb}, Code: "FC2-PPV-1234567"}
	junk, kept := classifyJunk([]pan.File{small.File, {ID: "h", Name: "a.html"}}, []Video{small}, 100*mb)
	if len(kept) != 1 || kept[0].ID != "small" {
		t.Fatalf("kept = %+v", kept)
	}
	if len(junk) != 1 || junk[0].ID != "h" {
		t.Fatalf("junk = %+v", junk)
	}
}

func TestClassifyJunkNoThresholdKeepsAllVideos(t *testing.T) {
	main := Video{File: pan.File{ID: "main", Name: "A.mp4", Size: 2000 * mb}}
	ad := Video{File: pan.File{ID: "ad", Name: "ad.mp4", Size: 5 * mb}}
	junk, kept := classifyJunk([]pan.File{main.File, ad.File}, []Video{main, ad}, 0)
	if len(junk) != 0 || len(kept) != 2 {
		t.Fatalf("junk=%v kept=%v", junk, kept)
	}
}
