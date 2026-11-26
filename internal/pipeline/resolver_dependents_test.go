package pipeline

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/cache"
	"github.com/kaeawc/krit/internal/config"
	"github.com/kaeawc/krit/internal/diag"
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

var classNameRE = regexp.MustCompile(`class\s+(\w+)`)

// extendsViewRule flags a class whose supertype chain, resolved through the
// resolver across files, reaches View: the shape of ClickableViewAccessibility
// and the other resolver-backed rules #1778 found stale.
func extendsViewRule() *api.Rule {
	return api.FakeRule("ExtendsView", api.WithNodeTypes("class_declaration"), api.WithNeeds(api.NeedsResolver),
		api.WithCheck(func(ctx *api.Context) {
			m := classNameRE.FindStringSubmatch(ctx.File.FlatNodeText(ctx.Idx))
			if m == nil || ctx.Resolver == nil {
				return
			}
			seen := map[string]bool{}
			for queue := []string{m[1]}; len(queue) > 0; queue = queue[1:] {
				name := queue[0]
				if name == "View" || strings.HasSuffix(name, ".View") {
					ctx.Emit(scanner.Finding{File: ctx.File.Path, Line: 1, Col: 1, Message: m[1] + " is a View"})
					return
				}
				if seen[name] {
					continue
				}
				seen[name] = true
				if info := ctx.Resolver.ClassHierarchy(name); info != nil {
					queue = append(queue, info.Supertypes...)
				}
			}
		}))
}

// resolverHarness runs RunProject over one directory with a shared on-disk
// analysis cache, the way consecutive CLI runs do.
type resolverHarness struct {
	t     *testing.T
	dir   string
	rules []*api.Rule
}

func newResolverHarness(t *testing.T, withCrossFileRule bool, files map[string]string) *resolverHarness {
	t.Helper()
	h := &resolverHarness{t: t, dir: t.TempDir(), rules: []*api.Rule{extendsViewRule()}}
	if withCrossFileRule {
		// The default rule set always includes cross-file rules, so the code
		// index that narrows the invalidation is built.
		h.rules = append(h.rules, api.FakeRule("CrossNoop", api.WithNeeds(api.NeedsCrossFile), api.WithCheck(func(*api.Context) {})))
	}
	for name, body := range files {
		h.write(name, body)
	}
	return h
}

func (h *resolverHarness) write(name, body string) {
	h.t.Helper()
	path := filepath.Join(h.dir, name)
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		h.t.Fatal(err)
	}
}

var reanalyzedRE = regexp.MustCompile(`re-analyzing (\d+) cached files`)

// run returns the sorted "file: message" findings of one run and how many
// cached files the #1778 pass re-analyzed; useCache false is the --no-cache
// truth.
func (h *resolverHarness) run(useCache bool) (findings []string, reanalyzed int) {
	h.t.Helper()
	var verbose bytes.Buffer
	host := ProjectHostState{Reporter: &diag.Reporter{Verbose: &verbose}}
	if useCache {
		cachePath := filepath.Join(h.dir, ".krit-test", "krit.cache")
		if err := os.MkdirAll(filepath.Dir(cachePath), 0o755); err != nil {
			h.t.Fatal(err)
		}
		host.AnalysisCache = cache.Load(cachePath)
		host.AnalysisCacheFilePath = cachePath
		host.AnalysisCacheLookup = true
	}
	res, err := RunProject(context.Background(), ProjectInput{
		Args: ProjectArgs{Config: config.NewConfig(), Paths: []string{h.dir}, ActiveRules: h.rules, Format: "json", Version: "test"},
		Host: host,
	})
	if err != nil {
		h.t.Fatalf("RunProject: %v", err)
	}
	for _, f := range res.FinalFindings.Findings() {
		if f.Rule == "ExtendsView" {
			findings = append(findings, filepath.Base(f.File)+": "+f.Message)
		}
	}
	sort.Strings(findings)
	if m := reanalyzedRE.FindStringSubmatch(verbose.String()); m != nil {
		reanalyzed, _ = strconv.Atoi(m[1])
	}
	return findings, reanalyzed
}

func (h *resolverHarness) wantWarmMatchesCold(label string) (reanalyzed int) {
	h.t.Helper()
	warm, reanalyzed := h.run(true)
	cold, _ := h.run(false)
	if strings.Join(warm, "\n") != strings.Join(cold, "\n") {
		h.t.Fatalf("%s: warm findings %v differ from --no-cache %v", label, warm, cold)
	}
	return reanalyzed
}

const (
	baseViewIsView  = "package p\n\nopen class BaseView : View() {\n    fun helper(): Int = 1\n}\n"
	baseViewNotView = "package p\n\nopen class BaseView {\n    fun helper(): Int = 1\n}\n"
	myView          = "package p\n\nclass MyView : BaseView()\n"
	subView         = "package p\n\nclass SubView : MyView()\n"
	unrelated       = "package p\n\nclass Unrelated\n"
)

// #1778: a cached file's resolver-backed verdict must follow a declaration
// change in another file, both ways, transitively, and after a deletion;
// a body-only edit must not invalidate anything.
func TestResolverRuleCacheFollowsOtherFilesDeclarations(t *testing.T) {
	for _, withIndex := range []bool{true, false} {
		name := "with-code-index"
		if !withIndex {
			name = "without-code-index"
		}
		t.Run(name, func(t *testing.T) {
			h := newResolverHarness(t, withIndex, map[string]string{
				"BaseView.kt": baseViewIsView, "MyView.kt": myView, "SubView.kt": subView, "Unrelated.kt": unrelated,
			})
			if got, _ := h.run(true); len(got) != 3 {
				t.Fatalf("cold findings = %v, want BaseView, MyView and SubView", got)
			}
			if n := h.wantWarmMatchesCold("warm"); n != 0 {
				t.Fatalf("an unchanged warm run re-analyzed %d cached files", n)
			}

			h.write("BaseView.kt", strings.Replace(baseViewIsView, "= 1", "= 42", 1))
			if n := h.wantWarmMatchesCold("body-only edit"); n != 0 {
				t.Fatalf("a body-only edit re-analyzed %d dependents", n)
			}

			h.write("BaseView.kt", baseViewNotView)
			n := h.wantWarmMatchesCold("BaseView stops extending View")
			if n != 2 {
				// MyView, and SubView through it; Unrelated stays cached
				// (without a code index, the parsed files' tokens narrow it).
				t.Fatalf("re-analyzed %d cached files, want 2", n)
			}

			h.write("BaseView.kt", baseViewIsView)
			h.wantWarmMatchesCold("BaseView extends View again")

			// Supertypes swapped between two classes in BaseView.kt: the
			// file declares and references the same names as before.
			h.write("BaseView.kt", "package p\n\nopen class BaseView : Plain() {\n    fun helper(): Int = 1\n}\n\nopen class Plain : View()\n")
			h.wantWarmMatchesCold("supertype moved within BaseView.kt")
			h.write("BaseView.kt", "package p\n\nopen class BaseView {\n    fun helper(): Int = 1\n}\n\nopen class Plain : View()\n")
			h.wantWarmMatchesCold("supertype swapped back out")

			// A supertype edit and a body edit to the middle of the chain in
			// one run: SubView names only MyView, which is re-analyzed, not
			// cached, and must still carry the change through.
			h.write("BaseView.kt", baseViewIsView)
			h.wantWarmMatchesCold("restore")
			h.write("BaseView.kt", baseViewNotView)
			h.write("MyView.kt", strings.Replace(myView, "BaseView()", "BaseView() {\n    fun x() = 1\n}", 1))
			h.wantWarmMatchesCold("supertype edit plus a body edit in the chain")
			h.write("BaseView.kt", baseViewIsView)
			h.write("MyView.kt", strings.Replace(myView, "BaseView()", "BaseView() {\n    fun x() = 2\n}", 1))
			h.wantWarmMatchesCold("and back")

			if err := os.Remove(filepath.Join(h.dir, "BaseView.kt")); err != nil {
				t.Fatal(err)
			}
			h.wantWarmMatchesCold("BaseView deleted")
		})
	}
}
