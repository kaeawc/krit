package rules

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/kaeawc/krit/internal/android"
	api "github.com/kaeawc/krit/internal/rules/api"
)

// This file ports Lint's UnusedResourceDetector: a resource that nothing reachable from the manifest,
// the code or a keep list refers to is unused, and a resource referenced only by unused resources is too.

var (
	rCodeRefPattern    = regexp.MustCompile(`\bR\s*\.\s*([a-z]+)\s*\.\s*([A-Za-z0-9_]+)`)
	rImportAliasRe     = regexp.MustCompile(`(?m)^\s*import\s+[\w.]*\bR\s+as\s+(\w+)`)
	bindingNamePattern = regexp.MustCompile(`\b([A-Z][A-Za-z0-9]*Binding)\b`)
	// resRefPattern matches @type/name and @+type/name, with an optional package; a package of android is framework.
	resRefPattern = regexp.MustCompile(`@\+?\*?(?:([A-Za-z_][\w.]*):)?([a-z]+)/([\w.$]+)`)
)

// ComputeResourceCodeRefs returns the resources source files refer to as R.type.name and the view binding
// classes they mention. Comments are skipped; android.R references are not app resources.
func ComputeResourceCodeRefs(sources [][]byte) *api.ResourceCodeRefs {
	refs := &api.ResourceCodeRefs{Refs: map[string]bool{}, Bindings: map[string]bool{}}
	for _, src := range sources {
		text := stripSourceComments(string(src))
		pattern := rCodeRefPattern
		// `import com.example.strings.R as Strings` makes Strings.string.x a reference too.
		if aliases := rImportAliasRe.FindAllStringSubmatch(text, -1); len(aliases) > 0 {
			names := []string{"R"}
			for _, a := range aliases {
				names = append(names, regexp.QuoteMeta(a[1]))
			}
			pattern = regexp.MustCompile(`\b(?:` + strings.Join(names, "|") + `)\s*\.\s*([a-z]+)\s*\.\s*([A-Za-z0-9_]+)`)
		}
		for _, m := range pattern.FindAllStringSubmatchIndex(text, -1) {
			if m[0] >= len("android.") && text[m[0]-len("android."):m[0]] == "android." {
				continue
			}
			refs.Refs[text[m[2]:m[3]]+"/"+resourceFieldName(text[m[4]:m[5]])] = true
		}
		if bytes.Contains(src, []byte("Binding")) {
			for _, m := range bindingNamePattern.FindAllStringSubmatch(text, -1) {
				refs.Bindings[m[1]] = true
			}
		}
	}
	return refs
}

// stripSourceComments blanks // and /* */ comments so references in them do not count. String,
// raw string and char literals are copied through, so "image/*" does not start a comment.
func stripSourceComments(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(s); {
		switch {
		case strings.HasPrefix(s[i:], "//"):
			for i < len(s) && s[i] != '\n' {
				i++
			}
		case strings.HasPrefix(s[i:], "/*"):
			end := strings.Index(s[i+2:], "*/")
			if end < 0 {
				return b.String()
			}
			i += end + 4
			b.WriteByte(' ')
		case strings.HasPrefix(s[i:], `"""`):
			end := strings.Index(s[i+3:], `"""`)
			n := len(s) - i
			if end >= 0 {
				n = end + 6
			}
			b.WriteString(s[i : i+n])
			i += n
		case s[i] == '"' || s[i] == '\'':
			n := quotedLiteralLen(s[i:])
			b.WriteString(s[i : i+n])
			i += n
		default:
			b.WriteByte(s[i])
			i++
		}
	}
	return b.String()
}

// quotedLiteralLen returns the length of the single-line string or char literal at the start of s.
// An unterminated literal ends at the line break.
func quotedLiteralLen(s string) int {
	quote := s[0]
	for i := 1; i < len(s); i++ {
		switch s[i] {
		case '\\':
			i++
		case quote:
			return i + 1
		case '\n':
			return i
		}
	}
	return len(s)
}

// resourceFieldName turns a resource name into its R field name.
func resourceFieldName(name string) string {
	return strings.NewReplacer(".", "_", "-", "_").Replace(name)
}

type unusedDecl struct {
	typ, name string
	path      string
	line      int
	ignored   bool
	order     string // sort key that puts default-config declarations first
}

type resourceGraph struct {
	decls map[string][]*unusedDecl // key type/name
	edges map[string][]string      // key -> referenced keys
	roots map[string]bool
	ids   map[string][]*unusedDecl // declared ids (UnusedIds)
}

// unusedReportedTypes are the resource types Lint's UnusedResources reports.
var unusedReportedTypes = map[string]bool{
	"string": true, "plurals": true, "array": true, "color": true, "dimen": true, "style": true, "bool": true,
	"integer": true, "fraction": true, "layout": true, "drawable": true, "mipmap": true, "menu": true,
	"anim": true, "animator": true, "xml": true, "raw": true, "interpolator": true, "transition": true,
	"navigation": true,
}

// fileResourceFolders are the res folder types whose files are resources named by the file.
var fileResourceFolders = map[string]bool{
	"layout": true, "drawable": true, "mipmap": true, "menu": true, "anim": true, "animator": true, "color": true,
	"xml": true, "raw": true, "interpolator": true, "transition": true, "navigation": true, "font": true,
}

func resKey(typ, name string) string { return typ + "/" + resourceFieldName(name) }

// scanRefs adds the resource references in text to refs, skipping framework ones.
func scanRefs(text string, refs map[string]bool) {
	for _, m := range resRefPattern.FindAllStringSubmatch(text, -1) {
		if m[1] == "android" {
			continue
		}
		refs[resKey(m[2], m[3])] = true
	}
}

func nodeRefs(node *android.XMLNode, refs map[string]bool) {
	for _, attr := range node.Attrs {
		// android:id="@+id/x" declares the id; every other mention refers to it.
		if attr.Name == "android:id" && strings.HasPrefix(attr.Value, "@+id/") {
			continue
		}
		scanRefs(attr.Value, refs)
	}
	scanRefs(node.Text, refs)
	for _, child := range node.Children {
		nodeRefs(child, refs)
	}
}

func toolsIgnoresUnused(node *android.XMLNode, issue string) bool {
	for _, attr := range node.Attrs {
		if attr.Name != "tools:ignore" {
			continue
		}
		for _, token := range strings.FieldsFunc(attr.Value, func(r rune) bool { return r == ',' || r == ' ' }) {
			if token == issue || token == "all" {
				return true
			}
		}
	}
	return false
}

func declOrder(folder, file string) string {
	qualified := "0"
	if strings.Contains(folder, "-") {
		qualified = "1"
	}
	return qualified + folder + "/" + file
}

// projectResourceGraph returns the graph of every res directory the project has, built once per scan.
func projectResourceGraph(ctx *api.Context, ignoreID string) *resourceGraph {
	return ctx.ResourceCodeRefs.Memoize(func() any { return buildResourceGraph(ctx, ignoreID) }).(*resourceGraph)
}

// inCurrentModules reports whether path sits under a res directory the current index was built from, so a
// rule that runs once per module reports each declaration once. Without that information every path counts.
func inCurrentModules(ctx *api.Context, path string) bool {
	dirs := ctx.ResourceIndex.ResDirs
	if len(dirs) == 0 {
		return true
	}
	for _, dir := range dirs {
		if strings.HasPrefix(path, dir+string(filepath.Separator)) {
			return true
		}
	}
	return false
}

// buildResourceGraph reads every res root's files and the manifest and build script beside it.
func buildResourceGraph(ctx *api.Context, ignoreID string) *resourceGraph {
	g := &resourceGraph{decls: map[string][]*unusedDecl{}, edges: map[string][]string{}, roots: map[string]bool{}, ids: map[string][]*unusedDecl{}}
	addEdges := func(from string, refs map[string]bool) {
		for to := range refs {
			if to != from {
				g.edges[from] = append(g.edges[from], to)
			}
		}
	}
	var keepPatterns []string
	for _, resRoot := range unusedResourceRoots(ctx) {
		moduleDir := filepath.Dir(resRoot)
		rootRefs := map[string]bool{}
		for _, extra := range []string{filepath.Join(moduleDir, "AndroidManifest.xml"), filepath.Join(filepath.Dir(moduleDir), "build.gradle"), filepath.Join(filepath.Dir(moduleDir), "build.gradle.kts"),
			filepath.Join(filepath.Dir(filepath.Dir(moduleDir)), "build.gradle"), filepath.Join(filepath.Dir(filepath.Dir(moduleDir)), "build.gradle.kts")} {
			if data, err := os.ReadFile(extra); err == nil {
				scanRefs(string(data), rootRefs)
			}
		}
		for k := range rootRefs {
			g.roots[k] = true
		}
		folders, err := os.ReadDir(resRoot)
		if err != nil {
			continue
		}
		for _, folderEntry := range folders {
			if !folderEntry.IsDir() {
				continue
			}
			folder := folderEntry.Name()
			folderType, _, _ := strings.Cut(folder, "-")
			files, _ := os.ReadDir(filepath.Join(resRoot, folder))
			for _, fileEntry := range files {
				if fileEntry.IsDir() {
					continue
				}
				path := filepath.Join(resRoot, folder, fileEntry.Name())
				switch {
				case folderType == "values" && strings.HasSuffix(fileEntry.Name(), ".xml"):
					scanValuesFile(path, folder, g, addEdges, ignoreID)
				case fileResourceFolders[folderType]:
					name, _, _ := strings.Cut(fileEntry.Name(), ".")
					decl := &unusedDecl{typ: folderType, name: name, path: path, line: 1, order: declOrder(folder, fileEntry.Name())}
					key := resKey(folderType, name)
					refs := map[string]bool{}
					if strings.HasSuffix(fileEntry.Name(), ".xml") {
						data, err := os.ReadFile(path)
						if err == nil {
							if root, perr := android.ParseXMLAST(context.Background(), data); perr == nil && root != nil {
								decl.ignored = toolsIgnoresUnused(root, ignoreID)
								nodeRefs(root, refs)
								collectIDDecls(root, path, g, ignoreID)
								if keep := root.Attr("tools:keep"); keep != "" && folderType == "raw" {
									keepPatterns = append(keepPatterns, strings.Split(keep, ",")...)
								}
							}
						}
					}
					g.decls[key] = append(g.decls[key], decl)
					addEdges(key, refs)
					// raw/keep.xml and raw/discard.xml configure the shrinker and are never "used".
					if folderType == "raw" && (name == "keep" || name == "discard") {
						g.roots[key] = true
					}
				}
			}
		}
	}
	for _, pattern := range keepPatterns {
		markKept(g, strings.TrimSpace(pattern))
	}
	return g
}

// markKept makes resources named by a tools:keep pattern such as @layout/l_* roots.
func markKept(g *resourceGraph, pattern string) {
	pattern = strings.TrimPrefix(pattern, "@")
	typ, name, ok := strings.Cut(pattern, "/")
	if !ok {
		return
	}
	re, err := regexp.Compile("^" + strings.ReplaceAll(regexp.QuoteMeta(resourceFieldName(name)), `\*`, ".*") + "$")
	if err != nil {
		return
	}
	for key, decls := range g.decls {
		if decls[0].typ == typ && re.MatchString(decls[0].name) {
			g.roots[key] = true
		}
	}
}

func scanValuesFile(path, folder string, g *resourceGraph, addEdges func(string, map[string]bool), ignoreID string) {
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	root, err := android.ParseXMLAST(context.Background(), data)
	if err != nil || root == nil || root.Tag != "resources" {
		return
	}
	for _, child := range root.Children {
		typ := child.Tag
		name := child.Attr("name")
		switch typ {
		case "string-array", "integer-array":
			typ = "array"
		case "item":
			typ = child.Attr("type")
		case "attr", "declare-styleable", "public", "eat-comment", "overlayable", "java-symbol", "skip":
			refs := map[string]bool{}
			nodeRefs(child, refs)
			for k := range refs {
				g.roots[k] = true
			}
			continue
		}
		if typ == "" || name == "" {
			continue
		}
		refs := map[string]bool{}
		nodeRefs(child, refs)
		key := resKey(typ, name)
		if typ == "style" {
			if parent := child.Attr("parent"); parent != "" {
				scanRefs("@style/"+strings.TrimPrefix(parent, "@style/"), refs)
			} else if i := strings.LastIndex(name, "."); i > 0 {
				refs[resKey("style", name[:i])] = true
			}
		}
		if typ == "id" {
			decl := &unusedDecl{typ: "id", name: name, path: path, line: child.Line, ignored: toolsIgnoresUnused(child, ignoreID), order: declOrder(folder, filepath.Base(path))}
			g.ids[key] = append(g.ids[key], decl)
			continue
		}
		decl := &unusedDecl{typ: typ, name: name, path: path, line: child.Line, ignored: toolsIgnoresUnused(child, "UnusedResources"), order: declOrder(folder, filepath.Base(path))}
		g.decls[key] = append(g.decls[key], decl)
		addEdges(key, refs)
	}
}

// collectIDDecls records the ids an XML tree declares with @+id/ in android:id attributes.
func collectIDDecls(node *android.XMLNode, path string, g *resourceGraph, ignoreID string) {
	if v := node.Attr("android:id"); strings.HasPrefix(v, "@+id/") {
		key := resKey("id", strings.TrimPrefix(v, "@+id/"))
		g.ids[key] = append(g.ids[key], &unusedDecl{typ: "id", name: strings.TrimPrefix(v, "@+id/"), path: path, line: node.Line,
			ignored: toolsIgnoresUnused(node, ignoreID), order: declOrder(filepath.Base(filepath.Dir(path)), filepath.Base(path))})
	}
	for _, child := range node.Children {
		collectIDDecls(child, path, g, ignoreID)
	}
}

// reachable returns the keys reachable from the roots.
func (g *resourceGraph) reachable(extraRoots map[string]bool) map[string]bool {
	seen := map[string]bool{}
	var queue []string
	push := func(k string) {
		if !seen[k] {
			seen[k] = true
			queue = append(queue, k)
		}
	}
	for k := range g.roots {
		push(k)
	}
	for k := range extraRoots {
		push(k)
	}
	for key, decls := range g.decls {
		for _, d := range decls {
			if d.ignored {
				push(key)
			}
		}
	}
	for len(queue) > 0 {
		k := queue[0]
		queue = queue[1:]
		for _, to := range g.edges[k] {
			push(to)
		}
	}
	return seen
}

// layoutBindingName is the view binding class generated for a layout file.
func layoutBindingName(layout string) string {
	var b strings.Builder
	for _, part := range strings.Split(layout, "_") {
		if part != "" {
			b.WriteString(strings.ToUpper(part[:1]) + part[1:])
		}
	}
	return b.String() + "Binding"
}

func codeRoots(ctx *api.Context, g *resourceGraph) map[string]bool {
	roots := map[string]bool{}
	if ctx.ResourceCodeRefs == nil {
		return roots
	}
	for k := range ctx.ResourceCodeRefs.Refs {
		roots[k] = true
	}
	for key, decls := range g.decls {
		if decls[0].typ == "layout" && ctx.ResourceCodeRefs.Bindings[layoutBindingName(decls[0].name)] {
			roots[key] = true
		}
	}
	return roots
}

// firstLocalDecl is the default-config declaration among those in the modules this run covers, or nil.
func firstLocalDecl(ctx *api.Context, decls []*unusedDecl) *unusedDecl {
	var local []*unusedDecl
	for _, d := range decls {
		if inCurrentModules(ctx, d.path) {
			local = append(local, d)
		}
	}
	if len(local) == 0 {
		return nil
	}
	return firstDecl(local)
}

func firstDecl(decls []*unusedDecl) *unusedDecl {
	sorted := append([]*unusedDecl(nil), decls...)
	sort.SliceStable(sorted, func(i, j int) bool { return sorted[i].order < sorted[j].order })
	return sorted[0]
}

// UnusedResourcesRule mirrors Lint's UnusedResources.
type UnusedResourcesRule struct {
	ValuesResourceBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence.
func (r *UnusedResourcesRule) Confidence() float64 { return api.ConfidenceMedium }

func (r *UnusedResourcesRule) check(ctx *api.Context) {
	if ctx.ResourceCodeRefs == nil {
		return
	}
	g := projectResourceGraph(ctx, "UnusedResources")
	used := g.reachable(codeRoots(ctx, g))
	keys := make([]string, 0, len(g.decls))
	for key := range g.decls {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		decls := g.decls[key]
		if used[key] || !unusedReportedTypes[decls[0].typ] {
			continue
		}
		d := firstLocalDecl(ctx, decls)
		if d == nil {
			continue
		}
		ctx.Emit(baseFinding(d.path, d.line, r.BaseRule,
			"The resource `R."+d.typ+"."+resourceFieldName(d.name)+"` appears to be unused"))
	}
}

// UnusedIdsRule mirrors Lint's UnusedIds: an id declared with @+id/ or <item type="id"> that no code or
// resource refers to.
type UnusedIdsRule struct {
	ValuesResourceBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence.
func (r *UnusedIdsRule) Confidence() float64 { return api.ConfidenceMedium }

func (r *UnusedIdsRule) check(ctx *api.Context) {
	if ctx.ResourceCodeRefs == nil {
		return
	}
	g := projectResourceGraph(ctx, "UnusedIds")
	// Any @id/ reference or R.id use keeps the id; the graph's edges already include @+id/ uses in other attributes.
	used := g.reachable(codeRoots(ctx, g))
	refsAnywhere := map[string]bool{}
	for _, tos := range g.edges {
		for _, to := range tos {
			refsAnywhere[to] = true
		}
	}
	keys := make([]string, 0, len(g.ids))
	for key := range g.ids {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		decls := g.ids[key]
		if used[key] || refsAnywhere[key] {
			continue
		}
		d := firstLocalDecl(ctx, decls)
		if d == nil || d.ignored {
			continue
		}
		ctx.Emit(baseFinding(d.path, d.line, r.BaseRule, "The resource `R.id."+resourceFieldName(d.name)+"` appears to be unused"))
	}
}

// unusedResourceRoots lists the res directories to read: those the index was built from, plus any it only
// saw through indexed files.
func unusedResourceRoots(ctx *api.Context) []string {
	idx := ctx.ResourceIndex
	seen := map[string]bool{}
	var roots []string
	all := append(append([]string(nil), ctx.ResourceCodeRefs.ResRoots...), idx.ResDirs...)
	for _, dir := range append(all, resourceRootsFromIndex(idx)...) {
		if !seen[dir] {
			seen[dir] = true
			roots = append(roots, dir)
		}
	}
	sort.Strings(roots)
	return roots
}
