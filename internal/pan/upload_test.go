package pan

import "testing"

func TestOSSSignature(t *testing.T) {
	got := ossSignature("test-secret", "Wed, 21 Oct 2015 07:28:00 GMT", map[string]string{
		"x-oss-security-token": "token123",
		"x-oss-callback":       "Y2I=",
	}, "mybucket", "path/to/file.nfo")
	const want = "j9XeUoIsiQBKAeVfegF38WdQ+a8="
	if got != want {
		t.Fatalf("ossSignature = %q, want %q", got, want)
	}
}

func TestParseSignCheck(t *testing.T) {
	start, end, err := parseSignCheck("10-20", 100)
	if err != nil || start != 10 || end != 20 {
		t.Fatalf("parseSignCheck valid = %d, %d, %v", start, end, err)
	}
	for _, bad := range []string{"", "abc", "5-3", "0-100", "-1-5"} {
		if _, _, err := parseSignCheck(bad, 100); err == nil {
			t.Fatalf("parseSignCheck(%q) succeeded, want error", bad)
		}
	}
}

func TestEscapeObjectKey(t *testing.T) {
	if got := escapeObjectKey("a b/c#d/e"); got != "a%20b/c%23d/e" {
		t.Fatalf("escapeObjectKey = %q", got)
	}
}
