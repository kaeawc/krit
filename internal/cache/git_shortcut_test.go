package cache

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/scanner"
)

// gitShortcutRepo is a one-file git repository whose cache records findings
// through the same CheckFiles / UpdateEntryColumns / Save / Load cycle a
// scan uses.
type gitShortcutRepo struct {
	t    *testing.T
	dir  string
	file string
}

func newGitShortcutRepo(t *testing.T, content string) *gitShortcutRepo {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	r := &gitShortcutRepo{t: t, dir: t.TempDir()}
	r.file = filepath.Join(r.dir, "A.kt")
	runGit(t, r.dir, "init", "-q")
	runGit(t, r.dir, "config", "user.email", "test@example.com")
	runGit(t, r.dir, "config", "user.name", "Test User")
	r.write(content)
	r.commit("seed")
	return r
}

func (r *gitShortcutRepo) write(content string) {
	r.t.Helper()
	if err := os.WriteFile(r.file, []byte(content), 0o644); err != nil {
		r.t.Fatal(err)
	}
}

func (r *gitShortcutRepo) commit(msg string) {
	r.t.Helper()
	runGit(r.t, r.dir, "add", "-A")
	runGit(r.t, r.dir, "commit", "-q", "--allow-empty", "-m", msg)
}

// record runs a scan that analyzes A.kt and reports one finding tagged
// message, then saves the cache and loads it back the way the next run does.
func (r *gitShortcutRepo) record(c *Cache, message string) *Cache {
	r.t.Helper()
	if c == nil {
		c = &Cache{RuleHash: "rh", ScanPaths: []string{r.dir}, Files: map[string]FileEntry{}}
	}
	c.CheckFiles([]string{r.file}, "rh", r.dir)
	cols := testFindingColumns([]scanner.Finding{{
		File: r.file, Line: 1, Col: 1, Severity: "warning", RuleSet: "test", Rule: "R", Message: message,
	}})
	c.UpdateEntryColumns(r.file, &cols)
	cachePath := filepath.Join(r.t.TempDir(), CacheFileName)
	if err := c.Save(cachePath); err != nil {
		r.t.Fatal(err)
	}
	loaded := Load(cachePath)
	loaded.RuleHash, loaded.ScanPaths = "rh", []string{r.dir}
	return loaded
}

func (r *gitShortcutRepo) cached(c *Cache) bool {
	r.t.Helper()
	result := c.CheckFiles([]string{r.file}, "rh", r.dir)
	if !result.GitDirtyPathsKnown {
		r.t.Fatal("git state was not read")
	}
	return result.CachedPaths[r.file]
}

// #1521: a cache written at one commit and used at another (a restored CI
// cache, or a git checkout / pull between runs) must not replay the old
// commit's findings for a file that changed between them, even though the
// file is clean against the new HEAD.
func TestGitShortcutMissesFileChangedByAnotherCommit(t *testing.T) {
	r := newGitShortcutRepo(t, "fun a() = 1\n")
	c := r.record(nil, "commit A")
	r.write("fun a(): Int = 2\n")
	r.commit("B")
	if r.cached(c) {
		t.Fatal("commit A's findings were replayed for a file commit B changed")
	}
}

// An entry written while the file was dirty describes the dirty content, not
// the index; reverting the file must not make that entry look clean.
func TestGitShortcutMissesEntryWrittenWhileDirty(t *testing.T) {
	r := newGitShortcutRepo(t, "fun a() = 1\n")
	r.write("fun a(): Int = 2\n")
	c := r.record(nil, "dirty content")
	runGit(t, r.dir, "checkout", "--", "A.kt")
	if r.cached(c) {
		t.Fatal("findings for the dirty content were replayed after the file was reverted")
	}
}

// git diff does not see edits to an --assume-unchanged file, so the index
// cannot vouch for its content.
func TestGitShortcutMissesAssumeUnchangedEdit(t *testing.T) {
	r := newGitShortcutRepo(t, "fun a() = 1\n")
	c := r.record(nil, "original")
	runGit(t, r.dir, "update-index", "--assume-unchanged", "A.kt")
	r.write("fun a(): Int = 2\n")
	if r.cached(c) {
		t.Fatal("an edited --assume-unchanged file was served from the cache")
	}
}

// The shortcut still serves unchanged files without a stat or a hash, and a
// staged file whose worktree matches the index hits too.
func TestGitShortcutHitsUnchangedAndStagedFiles(t *testing.T) {
	r := newGitShortcutRepo(t, "fun a() = 1\n")
	c := r.record(nil, "clean")
	poisonStat(t, c, r.file)
	if !r.cached(c) {
		t.Fatal("an unchanged clean file must hit through the git shortcut")
	}

	r.write("fun a(): Int = 2\n")
	runGit(t, r.dir, "add", "A.kt")
	c = r.record(c, "staged")
	poisonStat(t, c, r.file)
	if !r.cached(c) {
		t.Fatal("a staged file whose worktree matches the index must hit through the git shortcut")
	}
}

// poisonStat breaks the entry's stat and content hash, so only the git
// shortcut can produce a hit.
func poisonStat(t *testing.T, c *Cache, file string) {
	t.Helper()
	abs, _ := filepath.Abs(file)
	e, ok := c.Files[abs]
	if !ok {
		t.Fatalf("no entry for %s", abs)
	}
	e.ModTime, e.Size, e.Hash = 1, 1, "poisoned"
	c.Files[abs] = e
}

func TestGitBlobIDMatchesGitHashObject(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	content := []byte("fun a() = 1\n")
	for _, format := range []string{"sha1", "sha256"} {
		dir := t.TempDir()
		if out, err := exec.Command("git", "init", "-q", "--object-format="+format, dir).CombinedOutput(); err != nil {
			t.Logf("git init --object-format=%s unsupported: %v %s", format, err, out)
			continue
		}
		cmd := exec.Command("git", "hash-object", "--stdin")
		cmd.Dir = dir
		cmd.Stdin = bytes.NewReader(content)
		out, err := cmd.Output()
		if err != nil {
			t.Fatal(err)
		}
		want := strings.TrimSpace(string(out))
		if got := gitBlobID(content, len(want)); got != want {
			t.Errorf("%s: gitBlobID = %s, git hash-object = %s", format, got, want)
		}
	}
}

func TestParseIndexRecordVouchesOnlyForCleanRegularFiles(t *testing.T) {
	const blob = "78981922613b2afb6025042ff6bd878ac1994e85"
	for _, tc := range []struct {
		rec  string
		want bool
	}{
		{"H 100644 " + blob + " 0\tsrc/A.kt", true},
		{"H 100755 " + blob + " 0\tgradlew", true},
		{"H 100644 " + strings.Repeat("a", 64) + " 0\tsha256.kt", true},
		{"h 100644 " + blob + " 0\tassumed.kt", false},  // --assume-unchanged
		{"S 100644 " + blob + " 0\tsparse.kt", false},   // skip-worktree
		{"M 100644 " + blob + " 2\tconflict.kt", false}, // unmerged
		{"H 100644 " + blob + " 1\tconflict.kt", false}, // unmerged stage
		{"H 120000 " + blob + " 0\tlink.kt", false},     // symlink: blob is the target
		{"H 160000 " + blob + " 0\tsubmodule", false},   // gitlink
		{"H 100644 deadbeef 0\tshort.kt", false},        // not an object ID
		{"H 100644 " + blob + " 0\t", false},            // no path
		{"garbage", false},
	} {
		if _, _, ok := parseIndexRecord(tc.rec); ok != tc.want {
			t.Errorf("parseIndexRecord(%q) ok = %v, want %v", tc.rec, ok, tc.want)
		}
	}
	if rel, got, _ := parseIndexRecord("H 100644 " + blob + " 0\tdir with space/A.kt"); rel != "dir with space/A.kt" || got != blob {
		t.Errorf("path or blob mangled: %q %q", rel, got)
	}
}

func TestFileEntryJSONRoundTripsGitBlob(t *testing.T) {
	in := FileEntry{Hash: "h", ModTime: 1, Size: 2, GitBlob: "78981922613b2afb6025042ff6bd878ac1994e85"}
	data, err := in.MarshalJSON()
	if err != nil {
		t.Fatal(err)
	}
	var out FileEntry
	if err := out.UnmarshalJSON(data); err != nil {
		t.Fatal(err)
	}
	if out.GitBlob != in.GitBlob {
		t.Fatalf("GitBlob = %q after a JSON round trip, want %q", out.GitBlob, in.GitBlob)
	}
}

// Cache entries are keyed by filepath.Abs of the scanned path, which keeps
// a symlink; git reports the resolved top. Scanning through a symlinked
// path must still take the shortcut.
func TestGitShortcutHitsThroughASymlinkedScanRoot(t *testing.T) {
	r := newGitShortcutRepo(t, "fun a() = 1\n")
	link := filepath.Join(t.TempDir(), "link")
	if err := os.Symlink(r.dir, link); err != nil {
		t.Skip("symlinks unavailable:", err)
	}
	via := &gitShortcutRepo{t: t, dir: link, file: filepath.Join(link, "A.kt")}
	c := via.record(nil, "clean")
	poisonStat(t, c, via.file)
	if !via.cached(c) {
		t.Fatal("a clean file scanned through a symlinked root missed the git shortcut")
	}
}

func TestUserSpelledTop(t *testing.T) {
	sep := string(filepath.Separator)
	top := sep + filepath.Join("real", "repo")
	for _, tc := range []struct{ rawRoot, root, want string }{
		{sep + filepath.Join("link"), top, sep + "link"},
		{sep + filepath.Join("link", "app"), filepath.Join(top, "app"), sep + "link"},
		{sep + filepath.Join("elsewhere", "x"), filepath.Join(top, "app"), ""},
		{sep + "x", sep + filepath.Join("outside"), ""},
	} {
		if got := userSpelledTop(tc.rawRoot, tc.root, top); got != tc.want {
			t.Errorf("userSpelledTop(%q, %q) = %q, want %q", tc.rawRoot, tc.root, got, tc.want)
		}
	}
}
