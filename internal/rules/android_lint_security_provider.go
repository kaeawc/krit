package rules

import (
	"reflect"
	"strconv"
	"strings"

	"github.com/kaeawc/krit/internal/filefacts"
	"github.com/kaeawc/krit/internal/manifest"
	"github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// ProviderReadPermissionOnlyRule mirrors Lint's ProviderReadPermissionOnly (ProviderPermissionDetector): a manifest
// <provider> protected only by android:readPermission whose ContentProvider class implements insert, update or
// delete.
type ProviderReadPermissionOnlyRule struct {
	FlatDispatchBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence: the provider class and its superclass chain are resolved
// from project sources and imports.
func (r *ProviderReadPermissionOnlyRule) Confidence() float64 { return api.ConfidenceMedium }

// contentProviderClasses are the platform and library ContentProvider subclasses a project provider can extend.
var contentProviderClasses = lintStringSet(
	"android.content.ContentProvider",
	"android.provider.DocumentsProvider",
	"android.content.SearchRecentSuggestionsProvider",
	"androidx.core.content.FileProvider",
	"androidx.slice.SliceProvider",
	"androidx.startup.InitializationProvider",
)

// lintSourceClass is a class declared in project sources, with what is needed to walk its superclass chain.
type lintSourceClass struct {
	decl *preferenceClassDecl
	file *scanner.File
	idx  uint32
}

// lintSourceIndex indexes the classes declared in project sources for one rule run. Every lookup is a map access:
// the manifest-source rules query it once per component, class and intent reference, so nothing here may scan the
// whole project per call (#1658).
type lintSourceIndex struct {
	classes map[string]*lintSourceClass
	// byFQN holds the same classes as the preferenceClassDecl map resolvePreferenceSuper takes.
	byFQN map[string]*preferenceClassDecl
	// bySimple maps a class's simple name to the FQNs that end with it, for manifests without a package.
	bySimple map[string][]string
	// files memoizes each file's lintFileClasses for this index; cache, when not nil, shares them across a run.
	files map[*scanner.File]*lintFileClasses
	cache *filefacts.Cache
	// extends memoizes extendsTarget by class and target set.
	extends map[lintExtendsKey]bool
}

// lintExtendsKey identifies one extends query: a class FQN and a target set, by map identity. The target sets are
// package-level, so their identity is stable for the whole run.
type lintExtendsKey struct {
	fqn     string
	targets uintptr
}

// lintFileClasses is what one file contributes to a lintSourceIndex: its package and imports, and its class
// declarations in walk order. It is a per-file fact, so the manifest-source rules, which run concurrently, parse
// each file's imports once per run rather than once each.
type lintFileClasses struct {
	scope   *preferenceFileScope
	classes []*lintSourceClass
}

// lintSourceFileClasses returns the file's lintFileClasses from cache, computing it on first use. A nil cache
// computes it without keeping it.
func lintSourceFileClasses(cache *filefacts.Cache, file *scanner.File) *lintFileClasses {
	return filefacts.FileFact(cache, file, slotLintSourceFileClasses, func() *lintFileClasses {
		out := &lintFileClasses{scope: newPreferenceFileScope(string(file.Content))}
		file.FlatWalkNodes(0, "class_declaration", func(n uint32) {
			name := extractIdentifierFlat(file, n)
			if name == "" {
				return
			}
			d := &preferenceClassDecl{fqn: preferenceClassFQN(file, n, name, out.scope.pkg),
				supers: preferenceDirectSupers(file, n), file: out.scope}
			out.classes = append(out.classes, &lintSourceClass{decl: d, file: file, idx: n})
		})
		return out
	})
}

// lintSourceClasses indexes every class declared in the files by its FQN (nested classes joined with dots),
// sharing each file's parsed classes through the run's file-facts cache.
func lintSourceClasses(files []*scanner.File) *lintSourceIndex {
	return lintSourceClassesIn(fileFactsCache(), files)
}

// lintSourceClassesIn is lintSourceClasses with an explicit cache; nil keeps nothing beyond the index.
func lintSourceClassesIn(cache *filefacts.Cache, files []*scanner.File) *lintSourceIndex {
	idx := &lintSourceIndex{classes: map[string]*lintSourceClass{}, byFQN: map[string]*preferenceClassDecl{},
		bySimple: map[string][]string{}, files: make(map[*scanner.File]*lintFileClasses, len(files)),
		cache: cache, extends: map[lintExtendsKey]bool{}}
	for _, file := range files {
		if file == nil {
			continue
		}
		for _, c := range idx.fileClasses(file).classes {
			fqn := c.decl.fqn
			if idx.classes[fqn] == nil {
				simple := fqn[strings.LastIndex(fqn, ".")+1:]
				idx.bySimple[simple] = append(idx.bySimple[simple], fqn)
			}
			idx.classes[fqn] = c
			idx.byFQN[fqn] = c.decl
		}
	}
	return idx
}

func (idx *lintSourceIndex) fileClasses(file *scanner.File) *lintFileClasses {
	if fc := idx.files[file]; fc != nil {
		return fc
	}
	fc := lintSourceFileClasses(idx.cache, file)
	idx.files[file] = fc
	return fc
}

// scope returns the file's package and imports, parsed once per file.
func (idx *lintSourceIndex) scope(file *scanner.File) *preferenceFileScope {
	return idx.fileClasses(file).scope
}

// classAt returns the indexed class declared at a class_declaration node, or nil.
func (idx *lintSourceIndex) classAt(file *scanner.File, n uint32) *lintSourceClass {
	name := extractIdentifierFlat(file, n)
	if name == "" {
		return nil
	}
	return idx.classes[preferenceClassFQN(file, n, name, idx.scope(file).pkg)]
}

// resolve returns the indexed class a type name written in file refers to, by import, the file's package, a
// wildcard import, or a qualified name; nil when it is not a project class.
func (idx *lintSourceIndex) resolve(file *scanner.File, typ string) (string, *lintSourceClass) {
	typ = strings.TrimSpace(typ)
	if cut := strings.IndexByte(typ, '<'); cut >= 0 {
		typ = typ[:cut]
	}
	if typ == "" {
		return "", nil
	}
	scope := idx.scope(file)
	var candidates []string
	if strings.Contains(typ, ".") {
		first := typ[:strings.Index(typ, ".")]
		if imported, ok := scope.imports[first]; ok {
			candidates = append(candidates, imported+typ[len(first):])
		}
		candidates = append(candidates, typ)
	} else {
		if imported, ok := scope.imports[typ]; ok {
			candidates = append(candidates, imported)
		} else {
			if scope.pkg != "" {
				candidates = append(candidates, scope.pkg+"."+typ)
			} else {
				candidates = append(candidates, typ)
			}
			for _, wildcard := range scope.wildcards {
				candidates = append(candidates, wildcard+"."+typ)
			}
		}
	}
	for _, fqn := range candidates {
		if c := idx.classes[fqn]; c != nil {
			return fqn, c
		}
	}
	return "", nil
}

// extendsTarget reports whether the class's superclass chain, followed through project sources, reaches one of the
// targets.
func (idx *lintSourceIndex) extendsTarget(c *lintSourceClass, targets map[string]bool) bool {
	key := lintExtendsKey{fqn: c.decl.fqn, targets: reflect.ValueOf(targets).Pointer()}
	if v, ok := idx.extends[key]; ok {
		return v
	}
	seen := map[string]bool{}
	var reaches func(d *preferenceClassDecl) bool
	reaches = func(d *preferenceClassDecl) bool {
		if seen[d.fqn] {
			return false
		}
		seen[d.fqn] = true
		for _, super := range d.supers {
			fqn := resolvePreferenceSuper(d, super, idx.byFQN)
			if targets[fqn] {
				return true
			}
			if next := idx.byFQN[fqn]; next != nil && reaches(next) {
				return true
			}
		}
		return false
	}
	v := reaches(c.decl)
	idx.extends[key] = v
	return v
}

// lintManifestClass finds the project class a manifest component name refers to: the name resolved against the
// manifest package, or, when the manifest has no package (the Gradle namespace supplies it), the only class whose
// FQN ends with a relative name.
func lintManifestClass(m *manifest.Manifest, name string, classes *lintSourceIndex) (string, *lintSourceClass) {
	fqn := manifestClassFQN(m.Package, name)
	if c := classes.classes[fqn]; c != nil {
		return fqn, c
	}
	if m.Package != "" || (!strings.HasPrefix(name, ".") && strings.Contains(name, ".")) {
		return fqn, nil
	}
	suffix := strings.ReplaceAll(name, "$", ".")
	if !strings.HasPrefix(suffix, ".") {
		suffix = "." + suffix
	}
	// Every FQN ending with the suffix ends with its last segment, the class's simple name.
	var found *lintSourceClass
	foundFQN := ""
	for _, candidate := range classes.bySimple[suffix[strings.LastIndex(suffix, ".")+1:]] {
		if strings.HasSuffix(candidate, suffix) {
			if found != nil {
				return fqn, nil
			}
			found, foundFQN = classes.classes[candidate], candidate
		}
	}
	return foundFQN, found
}

func (r *ProviderReadPermissionOnlyRule) check(ctx *api.Context) {
	classes := lintSourceClasses(ctx.ParsedFiles)
	for _, m := range ctx.Manifests {
		if m == nil || m.Root == nil {
			continue
		}
		for _, app := range m.Root.ChildrenByTag("application") {
			for _, provider := range app.ChildrenByTag("provider") {
				r.checkProvider(ctx, m, provider, classes)
			}
		}
	}
}

func (r *ProviderReadPermissionOnlyRule) checkProvider(ctx *api.Context, m *manifest.Manifest, provider *manifest.Node,
	classes *lintSourceIndex) {
	read, ok := provider.Attr("android:readPermission")
	if !ok {
		return
	}
	if _, ok := provider.Attr("android:writePermission"); ok {
		return
	}
	if _, ok := provider.Attr("android:permission"); ok {
		return
	}
	fqn, cls := lintManifestClass(m, provider.AttrValue("android:name"), classes)
	if cls == nil || isProjectTestFile(ctx, cls.file.Path) || !classes.extendsTarget(cls, contentProviderClasses) {
		return
	}
	methods := providerImplementedWriteMethods(cls.file, cls.idx)
	if len(methods) == 0 {
		return
	}
	quoted := make([]string, len(methods))
	for i, name := range methods {
		quoted[i] = "`" + name + "`"
	}
	ctx.Emit(scanner.Finding{File: m.Path, Line: read.Line, Col: read.Col, Message: fqn + " implements {" +
		strings.Join(quoted, ", ") + "} write APIs but does not protect them with a permission. Update the " +
		"<provider> tag to use android:permission or android:writePermission"})
}

// providerWriteArity is the parameter count of each ContentProvider write method Lint looks for.
var providerWriteArity = map[string]int{"insert": 2, "update": 4, "delete": 3}

// providerImplementedWriteMethods lists, in declaration order, the insert/update/delete methods the class declares
// itself whose body does real work.
func providerImplementedWriteMethods(file *scanner.File, cls uint32) []string {
	var out []string
	javascriptInterfaceEachMember(file, cls, func(member uint32) {
		name := javascriptInterfaceMemberName(file, member)
		arity, ok := providerWriteArity[name]
		if !ok || providerParameterCount(file, member) != arity || !providerMethodImplemented(file, member) {
			return
		}
		out = append(out, name)
	})
	return out
}

func providerParameterCount(file *scanner.File, member uint32) int {
	params, ok := file.FlatFindChild(member, "function_value_parameters")
	if !ok {
		params, ok = file.FlatFindChild(member, "formal_parameters")
	}
	if !ok {
		return -1
	}
	n := 0
	for child := file.FlatFirstChild(params); child != 0; child = file.FlatNextSib(child) {
		switch file.FlatType(child) {
		case "parameter", "formal_parameter", "spread_parameter":
			n++
		}
	}
	return n
}

// providerMethodImplemented ports ProviderPermissionDetector.isImplemented: a body with more than one statement, or
// one that is not a throw, a returned literal, or a call that never returns. An empty body counts as implemented.
func providerMethodImplemented(file *scanner.File, member uint32) bool {
	var stmts []uint32
	if body, ok := file.FlatFindChild(member, "function_body"); ok {
		text := strings.TrimSpace(file.FlatNodeText(body))
		if !strings.HasPrefix(text, "{") {
			// An expression body is the implicitly returned value.
			return !providerTrivialText(strings.TrimPrefix(text, "="))
		}
		stmts = providerBlockStatements(file, body)
	} else if body, ok := file.FlatFindChild(member, "block"); ok {
		stmts = providerBlockStatements(file, body)
	} else {
		// No body: an abstract or native declaration.
		return false
	}
	if len(stmts) != 1 {
		return true
	}
	text := strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(file.FlatNodeText(stmts[0])), ";"))
	if strings.HasPrefix(text, "return ") || text == "return" {
		return !providerTrivialText(strings.TrimPrefix(text, "return"))
	}
	return !providerTrivialText(text)
}

// providerBlockStatements lists the statements of a Kotlin or Java block.
func providerBlockStatements(file *scanner.File, block uint32) []uint32 {
	var out []uint32
	for child := file.FlatFirstChild(block); child != 0; child = file.FlatNextSib(child) {
		if !file.FlatIsNamed(child) {
			continue
		}
		switch file.FlatType(child) {
		case "statements":
			out = append(out, providerBlockStatements(file, child)...)
		case "line_comment", "multiline_comment", "block_comment":
		default:
			out = append(out, child)
		}
	}
	return out
}

// providerTrivialText reports whether expression text is a literal, a throw, or a call that never returns.
func providerTrivialText(text string) bool {
	text = strings.TrimSpace(text)
	for strings.HasPrefix(text, "(") && strings.HasSuffix(text, ")") {
		text = strings.TrimSpace(text[1 : len(text)-1])
	}
	switch {
	case text == "null" || text == "true" || text == "false":
		return true
	case strings.HasPrefix(text, "throw "):
		return true
	case len(text) >= 2 && text[0] == '"' && text[len(text)-1] == '"' && !strings.ContainsAny(text[1:len(text)-1], `"$\`):
		return true
	case len(text) >= 3 && text[0] == '\'' && text[len(text)-1] == '\'':
		return true
	}
	if _, err := strconv.ParseFloat(strings.TrimRight(strings.ReplaceAll(text, "_", ""), "LlFfDd"), 64); err == nil {
		return true
	}
	if v := strings.ReplaceAll(text, "_", ""); strings.HasPrefix(v, "0x") || strings.HasPrefix(v, "0X") {
		if _, err := strconv.ParseInt(strings.TrimRight(v[2:], "Ll"), 16, 64); err == nil {
			return true
		}
	}
	// Calls declared to return Nothing.
	for _, fn := range []string{"TODO(", "error(", "kotlin.TODO(", "kotlin.error(", "exitProcess(", "kotlin.system.exitProcess("} {
		if strings.HasPrefix(text, fn) && strings.HasSuffix(text, ")") {
			return true
		}
	}
	return false
}
