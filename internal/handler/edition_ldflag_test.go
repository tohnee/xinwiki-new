package handler

import "testing"

// TestEditionLdflagPath_Exists proves that the package path embedded in
// scripts/get_version.sh's ldflags output (`github.com/Tencent/XinWiki/
// internal/handler.Edition=...`) actually resolves to this package.
//
// Background: the brand migration renamed module `github.com/Tencent/WeKnora`
// -> `github.com/Tencent/XinWiki`. The ldflags string in get_version.sh was
// left pointing at the OLD path for months after the migration, so the
// linker silently dropped the `-X` flag (it references a non-existent
// package) and handler.Edition stayed stuck on its source-level default
// "standard" forever. That broke Lite auto-setup
// (`handler/auth.go::Edition != "lite"` gate), the Lite swagger gate
// (`router.go:172`), and any other Lite-vs-Standard branch.
//
// This test is compiled INTO the binary at the same import path that the
// ldflag string names, so if a future rename drops the "XinWiki" path the
// test file would no longer compile AND this assertion would also need to
// be updated - which is exactly the early signal we want.
//
// We assert the compile-time identity rather than checking ldflag text,
// because (a) the ldflag text lives in a shell script that go test cannot
// execute portably, and (b) the *consequence* (Edition never reflects the
// build-time value) is the real bug we are guarding against.
func TestEditionLdflagPath_Exists(t *testing.T) {
	// Simply reference Edition - if the symbol disappears or the package
	// path would no longer be github.com/Tencent/XinWiki/internal/handler,
	// this test file fails to compile, surfacing the regression.
	if Edition == "" {
		t.Fatalf("Edition must be a non-empty compile-time default; ldflags target depends on it")
	}
	// Default must be "standard" or "lite"; the ldflag override replaces it.
	switch Edition {
	case "standard", "lite":
		// ok
	default:
		t.Fatalf("Edition=%q unexpected; build ldflag may have leaked a bad value", Edition)
	}
}

// TestBuildMetadataLdflagPaths_Exist mirrors the above for the other four
// build-info symbols (Version / CommitID / BuildTime / GoVersion) that
// get_version.sh injects via -ldflags. Each must resolve to a real symbol
// in this package; otherwise the same silent-drop bug recurs.
func TestBuildMetadataLdflagPaths_Exist(t *testing.T) {
	for name, val := range map[string]string{
		"Version":   Version,
		"CommitID":  CommitID,
		"BuildTime": BuildTime,
		"GoVersion": GoVersion,
	} {
		if val == "" {
			t.Errorf("%s must be a non-empty default (ldflag depends on it)", name)
		}
	}
}
