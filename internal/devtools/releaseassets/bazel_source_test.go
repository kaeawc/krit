package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"io"
	"net/url"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"
)

func bazelSourceScript(t *testing.T) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("the release scripts run on Linux runners")
	}
	for _, tool := range []string{"bash", "git", "gzip"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Skipf("%s not available", tool)
		}
	}
	p, err := filepath.Abs(filepath.Join("..", "..", "..", "scripts", "release", "bazel-source-archive.sh"))
	if err != nil {
		t.Fatal(err)
	}
	return p
}

// gitRepo creates a repository with a bazel/ module and a file outside it,
// tagged as tag.
func gitRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	files := map[string]string{
		"bazel/MODULE.bazel":       "module(name = \"krit\", version = \"0.0.0\")\n",
		"bazel/private/rule.bzl":   "# rule\n",
		"bazel/e2e/MODULE.bazel":   "module(name = \"krit_e2e\")\n",
		"cmd/krit/main.go":         "package main\n",
		"README.md":                "# krit\n",
		"bazel-not-module/file.md": "outside bazel/\n",
	}
	for name, body := range files {
		p := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for _, args := range [][]string{
		{"init", "-q"},
		{"add", "."},
		{"-c", "user.name=t", "-c", "user.email=t@example.com", "commit", "-q", "-m", "init"},
		{"-c", "user.name=t", "-c", "user.email=t@example.com", "tag", "-a", tag, "-m", tag},
	} {
		runIn(t, dir, "git", args...)
	}
	return dir
}

func runIn(t *testing.T, dir, name string, args ...string) string {
	t.Helper()
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_NOSYSTEM=1")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%s %v: %v\n%s", name, args, err, out)
	}
	return string(out)
}

func tarNames(t *testing.T, data []byte) []string {
	t.Helper()
	gz, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	tr := tar.NewReader(gz)
	var names []string
	for {
		h, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		if h.Typeflag == tar.TypeReg {
			names = append(names, h.Name)
		}
	}
	sort.Strings(names)
	return names
}

// The script archives only bazel/, under the prefix the template strips, and
// a rebuild of the same tag is byte-identical, so the integrity the registry
// records stays valid.
func TestBazelSourceArchiveScript(t *testing.T) {
	script := bazelSourceScript(t)
	repo := gitRepo(t)

	var archives [][]byte
	for i := 0; i < 2; i++ {
		out := t.TempDir()
		runIn(t, repo, "bash", script, tag, out)
		data, err := os.ReadFile(filepath.Join(out, bazelArchiveName(tag)))
		if err != nil {
			t.Fatal(err)
		}
		archives = append(archives, data)
	}
	if !bytes.Equal(archives[0], archives[1]) {
		t.Fatal("two builds of the same tag differ")
	}
	if err := checkTarGzHas(archives[0], bazelModuleFile(tag)); err != nil {
		t.Fatal(err)
	}
	want := []string{
		"krit-bazel-v1.2.3/bazel/MODULE.bazel",
		"krit-bazel-v1.2.3/bazel/e2e/MODULE.bazel",
		"krit-bazel-v1.2.3/bazel/private/rule.bzl",
	}
	if got := tarNames(t, archives[0]); strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("archive files = %q, want %q", got, want)
	}
}

func TestBazelSourceArchiveScriptRejectsUnknownTag(t *testing.T) {
	script := bazelSourceScript(t)
	repo := gitRepo(t)
	cmd := exec.Command("bash", script, "v9.9.9", t.TempDir())
	cmd.Dir = repo
	out, err := cmd.CombinedOutput()
	if err == nil || !strings.Contains(string(out), "tag v9.9.9 not found") {
		t.Fatalf("unknown tag: err = %v, output %q", err, out)
	}
}

// .bcr/bazel/source.template.json must download the archive the release
// uploads, from a URL BCR treats as stable, and strip to the module root.
func TestBCRSourceTemplateMatchesReleaseArchive(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "..", ".bcr", "bazel", "source.template.json"))
	if err != nil {
		t.Fatal(err)
	}
	var tmpl struct {
		URL         string `json:"url"`
		StripPrefix string `json:"strip_prefix"`
	}
	if err := json.Unmarshal(data, &tmpl); err != nil {
		t.Fatal(err)
	}
	// publish-to-bcr substitutes these in url and strip_prefix.
	sub := strings.NewReplacer("{OWNER}", "kaeawc", "{REPO}", "krit", "{TAG}", tag, "{VERSION}", strings.TrimPrefix(tag, "v"))

	gotURL := sub.Replace(tmpl.URL)
	wantURL := "https://github.com/kaeawc/krit/releases/download/" + tag + "/" + bazelArchiveName(tag)
	if gotURL != wantURL {
		t.Fatalf("template url resolves to %s, want the release asset %s", gotURL, wantURL)
	}
	// BCR's verify_stable_archive: only github.com/<owner>/<repo>/releases/download/... is stable.
	u, err := url.Parse(gotURL)
	if err != nil {
		t.Fatal(err)
	}
	parts := strings.Split(u.Path, "/")
	if u.Scheme != "https" || u.Host != "github.com" || len(parts) < 5 || parts[3] != "releases" || parts[4] != "download" {
		t.Fatalf("%s is not a URL BCR treats as stable", gotURL)
	}
	if got, want := sub.Replace(tmpl.StripPrefix), path.Dir(bazelModuleFile(tag)); got != want {
		t.Fatalf("template strip_prefix resolves to %s, want %s", got, want)
	}
}
