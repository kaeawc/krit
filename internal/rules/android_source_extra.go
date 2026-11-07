package rules

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	"github.com/kaeawc/krit/internal/android"
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

type ViewConstructorRule struct {
	FlatDispatchBase
	AndroidRule
}

// viewConstructedInFile reports a call that builds className in code, such as `PlainView(context, 1)`, which
// never needs the XML-inflation constructor.
func viewConstructedInFile(file *scanner.File, className string) bool {
	found := false
	file.FlatWalkNodes(0, "call_expression", func(call uint32) {
		if !found && flatCallExpressionName(file, call) == className {
			found = true
		}
	})
	return found
}

// viewConstructorClassFQN returns the dotted name of the class declaration, nesting included.
func viewConstructorClassFQN(file *scanner.File, idx uint32) string {
	pkg := ""
	if m := sourcePackageRe.FindStringSubmatch(string(file.Content)); m != nil {
		pkg = m[1]
	}
	return preferenceClassFQN(file, idx, extractIdentifierFlat(file, idx), pkg)
}

var (
	viewClassFactsMu    sync.Mutex
	viewClassFactsIndex *android.ResourceIndex
	viewClassFactsSet   map[string]bool
)

// viewClassReferencedInLayouts reports whether layout XML names the class by FQN or bare simple name.
func viewClassReferencedInLayouts(idx *android.ResourceIndex, fqn string) bool {
	viewClassFactsMu.Lock()
	if viewClassFactsIndex != idx || viewClassFactsSet == nil {
		viewClassFactsIndex, viewClassFactsSet = idx, idx.ReferencedViewClasses()
	}
	set := viewClassFactsSet
	viewClassFactsMu.Unlock()
	return set[fqn] || set[fqn[strings.LastIndex(fqn, ".")+1:]]
}

// viewConstructorNeedsExtraArguments reports a primary constructor with a required parameter other than
// Context, AttributeSet or a style Int, when the class has no secondary constructor to fall back on.
func viewConstructorNeedsExtraArguments(file *scanner.File, idx uint32) bool {
	if body, ok := file.FlatFindChild(idx, "class_body"); ok {
		hasSecondary := false
		file.FlatWalkNodes(body, "secondary_constructor", func(uint32) { hasSecondary = true })
		if hasSecondary {
			return false
		}
	}
	ctor, ok := file.FlatFindChild(idx, "primary_constructor")
	if !ok {
		return false
	}
	extra := false
	file.FlatWalkNodes(ctor, "class_parameter", func(param uint32) {
		text := file.FlatNodeText(param)
		if strings.Contains(text, "=") {
			return
		}
		name, typ, _ := strings.Cut(text, ":")
		typ = strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(typ), "?"))
		switch {
		case typ == "Context" || typ == "AttributeSet" || strings.HasSuffix(typ, ".Context") || strings.HasSuffix(typ, ".AttributeSet"):
		case typ == "Int" && strings.Contains(strings.ToLower(name), "style"):
		default:
			extra = true
		}
	})
	return extra
}

// Confidence reports a tier-2 (medium) base confidence. This is an
// Android-lint port from AOSP; the detection relies on source-text
// patterns (call names, string literal contents, hardcoded allow-
// lists of API names) rather than type resolution, so project-
// specific wrapper APIs can cause false positives or negatives.
// Classified per roadmap/17.
func (r *ViewConstructorRule) Confidence() float64 { return api.ConfidenceMedium }

// MayDeclareCustomView is a cheap byte prefilter: the source names a platform view superclass.
func MayDeclareCustomView(content []byte) bool {
	for _, name := range viewSuperclasses {
		if bytes.Contains(content, []byte(name)) {
			return true
		}
	}
	return false
}

var viewSuperclasses = []string{"View", "ViewGroup", "TextView", "ImageView", "LinearLayout", "RelativeLayout", "FrameLayout", "ConstraintLayout", "RecyclerView", "SurfaceView", "EditText", "Button", "ScrollView", "HorizontalScrollView", "AppBarLayout", "CoordinatorLayout", "CardView", "Toolbar"}

func nodeAtPoint(file *scanner.File, line, column int) uint32 {
	if file == nil || file.FlatTree == nil || line <= 0 || line > len(file.Lines) {
		return 0
	}
	if column < 0 {
		column = 0
	}
	offset := file.LineOffset(line-1) + column
	if offset < 0 {
		offset = 0
	}
	if offset > len(file.Content) {
		offset = len(file.Content)
	}
	idx, ok := file.FlatNamedDescendantForByteRange(uint32(offset), uint32(offset))
	if !ok {
		return 0
	}
	return idx
}

func nodeAtLine(file *scanner.File, line int) uint32 {
	if file == nil || line <= 0 || line > len(file.Lines) {
		return 0
	}
	trimmed := strings.TrimLeft(file.Lines[line-1], " \t")
	column := len(file.Lines[line-1]) - len(trimmed)
	return nodeAtPoint(file, line, column)
}

func enclosingAncestor(file *scanner.File, idx uint32, types ...string) uint32 {
	for cur, ok := idx, idx != 0; ok; cur, ok = file.FlatParent(cur) {
		for _, typ := range types {
			if file.FlatType(cur) == typ {
				return cur
			}
		}
	}
	return 0
}

func prefixTextBeforeLine(file *scanner.File, line int, scopeTypes ...string) (string, bool) {
	scope := enclosingAncestor(file, nodeAtLine(file, line), scopeTypes...)
	if scope == 0 {
		return "", false
	}
	lineOffset := file.LineOffset(line - 1)
	start := int(file.FlatStartByte(scope))
	if lineOffset <= start {
		return "", false
	}
	return string(file.Content[start:lineOffset]), true
}

func declarationHeaderText(file *scanner.File, idx uint32) string {
	if idx == 0 {
		return ""
	}
	end := int(file.FlatEndByte(idx))
	if body, ok := file.FlatFindChild(idx, "class_body"); ok {
		end = int(file.FlatStartByte(body))
	}
	start := int(file.FlatStartByte(idx))
	if end <= start {
		return ""
	}
	return string(file.Content[start:end])
}

func containsAny(text string, needles []string) bool {
	for _, needle := range needles {
		if strings.Contains(text, needle) {
			return true
		}
	}
	return false
}

// containsAnyNonEmpty is containsAny that ignores empty needles, which would match every text.
func containsAnyNonEmpty(text string, needles []string) bool {
	for _, needle := range needles {
		if needle != "" && strings.Contains(text, needle) {
			return true
		}
	}
	return false
}

func viewConstructorSupertypeNameFlat(file *scanner.File, deleg uint32) string {
	ut, _ := file.FlatFindChild(deleg, "user_type")
	if ut == 0 {
		if ci, ok := file.FlatFindChild(deleg, "constructor_invocation"); ok {
			ut, _ = file.FlatFindChild(ci, "user_type")
		}
	}
	if ut == 0 {
		return ""
	}
	var lastIdent string
	for child := file.FlatFirstChild(ut); child != 0; child = file.FlatNextSib(child) {
		if file.FlatType(child) == "type_identifier" {
			lastIdent = file.FlatNodeText(child)
		}
	}
	return lastIdent
}

type WrongImportRule struct {
	FlatDispatchBase
	AndroidRule
}

// Confidence: tier-1 for this rule because the detection is purely
// syntactic — an import_header's `identifier` child carries the exact
// dotted FQN. No symbol resolution is needed and no source-text
// heuristics are involved.
func (r *WrongImportRule) Confidence() float64 { return api.ConfidenceVeryHigh }

// check runs per import_header node. It flags `import android.R` and
// any `import android.R.<anything>` (e.g. `android.R.layout`) because
// these refer to the framework R class rather than the application's
// generated R. Aliased imports (`import android.R as FR`) are flagged
// all the same — the alias is syntactic sugar; the underlying import
// is still wrong.
func (r *WrongImportRule) check(ctx *api.Context) {
	idx, file := ctx.Idx, ctx.File
	ident, ok := file.FlatFindChild(idx, "identifier")
	if !ok {
		return
	}
	fqn := file.FlatNodeText(ident)
	if fqn != "android.R" && !strings.HasPrefix(fqn, "android.R.") {
		return
	}
	// An alias (`import android.R as AndroidR`) keeps the framework R from shadowing the app's R.
	if file.FlatHasChildOfType(idx, "import_alias") {
		return
	}
	msg := "Don't include `android.R` here; use a fully qualified name for each usage instead"
	if sub := strings.TrimPrefix(fqn, "android.R."); sub != fqn {
		leaf := sub[strings.LastIndex(sub, ".")+1:]
		referenced := wrongImportNameReferenced(file, idx, leaf)
		// A sub-class or id import that nothing resolves to is an unused import, which UnusedImport reports.
		if !referenced {
			return
		}
		// A framework R sub-class (`android.R.layout`) that is referenced by name is a deliberate use of framework resources.
		if !strings.Contains(sub, ".") {
			return
		}
		msg = "Import '" + fqn + "' resolves to a framework R resource id, and `" + leaf + "` is used as a value in this file; a framework id is not the application's resource, so import the application's R instead."
	}
	f := r.Finding(file, file.FlatRow(idx)+1, file.FlatCol(idx)+1, msg)
	if pkg := sourcePackageName(file); pkg != "" && pkg != "android" {
		f.Fix = &scanner.Fix{
			ByteMode:    true,
			StartByte:   int(file.FlatStartByte(ident)),
			EndByte:     int(file.FlatEndByte(ident)),
			Replacement: pkg + ".R" + strings.TrimPrefix(fqn, "android.R"),
		}
	}
	ctx.Emit(f)
}

// wrongImportNameReferenced reports whether a bare identifier named name outside import headers can resolve to
// the imported symbol. Qualified selectors, named-argument labels, declaration names and identifiers shadowed by
// a parameter, local or member of the same name do not count.
func wrongImportNameReferenced(file *scanner.File, importIdx uint32, name string) bool {
	found := false
	file.FlatWalkNodes(0, "simple_identifier", func(n uint32) {
		if found || !file.FlatNodeTextEquals(n, name) {
			return
		}
		if _, inImport := flatEnclosingAncestor(file, n, "import_header"); inImport {
			return
		}
		parent, ok := file.FlatParent(n)
		if !ok {
			return
		}
		switch file.FlatType(parent) {
		case "navigation_suffix", "parameter", "class_parameter", "variable_declaration", "function_declaration", "type_parameter":
			return
		case "value_argument":
			if next, ok := file.FlatNextSibling(n); ok && file.FlatType(next) == "=" {
				return
			}
		}
		if wrongImportNameShadowed(file, n, name) {
			return
		}
		found = true
	})
	return found
}

// wrongImportNameShadowed reports whether an enclosing function, lambda or class declares name itself.
func wrongImportNameShadowed(file *scanner.File, ref uint32, name string) bool {
	for cur, ok := file.FlatParent(ref); ok; cur, ok = file.FlatParent(cur) {
		switch file.FlatType(cur) {
		case "function_declaration", "lambda_literal", "anonymous_function", "secondary_constructor", "class_declaration", "object_declaration":
		default:
			continue
		}
		declared := false
		for _, declType := range []string{"parameter", "class_parameter", "variable_declaration"} {
			file.FlatWalkNodes(cur, declType, func(d uint32) {
				if declared {
					return
				}
				for c := file.FlatFirstChild(d); c != 0; c = file.FlatNextSib(c) {
					if file.FlatType(c) == "simple_identifier" && file.FlatNodeTextEquals(c, name) {
						declared = true
						return
					}
				}
			})
		}
		if declared {
			return true
		}
	}
	return false
}

type InstantiatableRule struct {
	FlatDispatchBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence. This is an
// Android-lint port from AOSP; the detection relies on source-text
// patterns (call names, string literal contents, hardcoded allow-
// lists of API names) rather than type resolution, so project-
// specific wrapper APIs can cause false positives or negatives.
// Classified per roadmap/17.
func (r *InstantiatableRule) Confidence() float64 { return api.ConfidenceMedium }

// componentSuperclasses omits BroadcastReceiver: a receiver registered at runtime may be private.
var componentSuperclasses = []string{"Activity", "AppCompatActivity", "ComponentActivity", "FragmentActivity", "Service", "IntentService", "ContentProvider", "Application"}

type RtlAwareRule struct {
	FlatDispatchBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence. This is an
// Android-lint port from AOSP; the detection relies on source-text
// patterns (call names, string literal contents, hardcoded allow-
// lists of API names) rather than type resolution, so project-
// specific wrapper APIs can cause false positives or negatives.
// Classified per roadmap/17.
func (r *RtlAwareRule) Confidence() float64 { return api.ConfidenceMedium }

// rtlAwareMethods maps View member names to their RTL-aware replacements.
// Keys are bare callee identifiers; the rule uses flatCallExpressionName
// to match, so there is no need to encode leading `.` / trailing `(`.
var rtlAwareMethods = map[string]string{
	// View has no getStart()/getEnd(); an empty value means "no direct replacement".
	"getLeft":         "",
	"getRight":        "",
	"getPaddingLeft":  "getPaddingStart()",
	"getPaddingRight": "getPaddingEnd()",
}

type RtlFieldAccessRule struct {
	FlatDispatchBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence. This is an
// Android-lint port from AOSP; the detection relies on source-text
// patterns (call names, string literal contents, hardcoded allow-
// lists of API names) rather than type resolution, so project-
// specific wrapper APIs can cause false positives or negatives.
// Classified per roadmap/17.
func (r *RtlFieldAccessRule) Confidence() float64 { return api.ConfidenceMedium }

var rtlFieldNames = []string{"mLeft", "mRight", "mPaddingLeft", "mPaddingRight"}

type GridLayoutRule struct {
	FlatDispatchBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence. This is an
// Android-lint port from AOSP; the detection relies on source-text
// patterns (call names, string literal contents, hardcoded allow-
// lists of API names) rather than type resolution, so project-
// specific wrapper APIs can cause false positives or negatives.
// Classified per roadmap/17.
func (r *GridLayoutRule) Confidence() float64 { return api.ConfidenceHigh }

type MangledCRLFRule struct {
	LineBase
	AndroidRule
}

// Confidence bumps this line rule from the 0.75 line-rule default to
// 0.95 -- mixed line endings is a literal substring check on the
// file bytes. Deterministic with no heuristic path.
func (r *MangledCRLFRule) Confidence() float64 { return api.ConfidenceVeryHigh }

func (r *MangledCRLFRule) check(ctx *api.Context) {
	file := ctx.File
	content := string(file.Content)
	if strings.Contains(content, "\r\n") && strings.Contains(strings.ReplaceAll(content, "\r\n", ""), "\n") {
		ctx.Emit(r.Finding(file, 1, 1,
			"File has mixed line endings (both CRLF and LF). Use consistent line endings."))
		return
	}
}

type ResourceNameRule struct {
	FlatDispatchBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence. This is an
// Android-lint port from AOSP; the detection relies on source-text
// patterns (call names, string literal contents, hardcoded allow-
// lists of API names) rather than type resolution, so project-
// specific wrapper APIs can cause false positives or negatives.
// Classified per roadmap/17.
func (r *ResourceNameRule) Confidence() float64 { return api.ConfidenceHigher }

var snakeCaseRe = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)

// ProguardRule reports a reference to the obsolete proguard.cfg file in a Gradle build script.
type ProguardRule struct {
	GradleBase
	AndroidRule
}

func (r *ProguardRule) check(ctx *api.Context) {
	for i, line := range strings.Split(ctx.GradleContent, "\n") {
		if isGradleCommentLine(line) {
			continue
		}
		if c := gradleLineCommentStart(line); c >= 0 {
			line = line[:c]
		}
		if strings.Contains(line, "proguard.cfg") {
			ctx.Emit(baseFinding(ctx.GradlePath, i+1, r.BaseRule,
				"Reference to obsolete `proguard.cfg`. Use `proguard-rules.pro` instead."))
		}
	}
}

// Confidence reports a tier-2 (medium) base confidence. This is an
// Android-lint port from AOSP; the detection relies on source-text
// patterns (call names, string literal contents, hardcoded allow-
// lists of API names) rather than type resolution, so project-
// specific wrapper APIs can cause false positives or negatives.
// Classified per roadmap/17.
func (r *ProguardRule) Confidence() float64 { return api.ConfidenceMedium }

type ProguardSplitRule struct {
	LineBase
	AndroidRule
}

var (
	proguardGenericRe  = regexp.MustCompile(`-dontobfuscate|-dontoptimize|-dontwarn|-keepattributes`)
	proguardSpecificRe = regexp.MustCompile(`-keep\s+class\s+\w+`)
)

// Confidence reports a tier-2 (medium) base confidence. This is an
// Android-lint port from AOSP; the detection relies on source-text
// patterns (call names, string literal contents, hardcoded allow-
// lists of API names) rather than type resolution, so project-
// specific wrapper APIs can cause false positives or negatives.
// Classified per roadmap/17.
func (r *ProguardSplitRule) Confidence() float64 { return api.ConfidenceMediumLow }

func (r *ProguardSplitRule) check(ctx *api.Context) {
	file := ctx.File
	content := strings.Join(file.Lines, "\n")
	if !strings.Contains(content, "-keep") && !strings.Contains(content, "-dontwarn") {
		return
	}
	if proguardGenericRe.MatchString(content) && proguardSpecificRe.MatchString(content) {
		ctx.Emit(r.Finding(file, 1, 1, "Proguard configuration contains both generic Android rules and project-specific rules. Consider splitting into separate files for maintainability."))
		return
	}
}

// NfcTechWhitespaceRule reports whitespace around the name inside a <tech> element of an NFC tech-list resource
// (res/xml/*.xml), which the platform does not trim when matching.
type NfcTechWhitespaceRule struct {
	ValuesResourceBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence. This is an
// Android-lint port from AOSP; the detection relies on source-text
// patterns (call names, string literal contents, hardcoded allow-
// lists of API names) rather than type resolution, so project-
// specific wrapper APIs can cause false positives or negatives.
// Classified per roadmap/17.
func (r *NfcTechWhitespaceRule) Confidence() float64 { return api.ConfidenceMedium }

func (r *NfcTechWhitespaceRule) check(ctx *api.Context) {
	for _, resRoot := range resourceRootsFromIndex(ctx.ResourceIndex) {
		xmlDir := filepath.Join(resRoot, "xml")
		entries, err := os.ReadDir(xmlDir)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".xml") {
				continue
			}
			path := filepath.Join(xmlDir, entry.Name())
			data, err := os.ReadFile(path)
			if err != nil || !bytes.Contains(data, []byte("<tech")) {
				continue
			}
			root, err := android.ParseXMLAST(context.Background(), data)
			if err != nil {
				continue
			}
			r.checkNode(ctx, path, root)
		}
	}
}

func (r *NfcTechWhitespaceRule) checkNode(ctx *api.Context, path string, node *android.XMLNode) {
	if node == nil {
		return
	}
	// Lint looks only at a <tech> directly inside <tech-list> whose sole child is text.
	if node.Tag == "tech-list" {
		for _, tech := range node.Children {
			if tech.Tag == "tech" && len(tech.Children) == 0 && tech.Text != strings.TrimSpace(tech.Text) {
				ctx.Emit(baseFindingAt(path, tech.Line, tech.Col, r.BaseRule,
					"There should not be any whitespace inside `<tech>` elements"))
			}
		}
	}
	for _, child := range node.Children {
		r.checkNode(ctx, path, child)
	}
}

// LibraryCustomViewRule reports a hardcoded package namespace such as
// xmlns:app="http://schemas.android.com/apk/res/com.example" in a layout of an Android library module, where
// res-auto is required because the package of the consuming app differs.
type LibraryCustomViewRule struct {
	LayoutResourceBase
	AndroidRule
}

func (r *LibraryCustomViewRule) check(ctx *api.Context) {
	for _, layout := range ctx.ResourceIndex.AllLayouts() {
		if gi := lookupManifestGradleInfo(layout.FilePath); !gi.found || !gi.isLibrary {
			continue
		}
		walkViewsUnlessIgnored(layout.RootView, []string{"LibraryCustomView"}, func(v *android.View) {
			for attr, val := range v.Attributes {
				if !strings.HasPrefix(attr, "xmlns:") || !hardcodedNsRe.MatchString(val) ||
					strings.Contains(val, "apk/res-auto") || strings.HasSuffix(val, "apk/res/android") {
					continue
				}
				ctx.Emit(baseFindingAtAttr(layout.FilePath, v, attr, r.BaseRule,
					"Use `http://schemas.android.com/apk/res-auto` instead of the hardcoded package namespace `"+val+"`. Hardcoded namespaces don't work in library projects."))
			}
		})
	}
}

// Confidence reports a tier-2 (medium) base confidence. This is an
// Android-lint port from AOSP; the detection relies on source-text
// patterns (call names, string literal contents, hardcoded allow-
// lists of API names) rather than type resolution, so project-
// specific wrapper APIs can cause false positives or negatives.
// Classified per roadmap/17.
func (r *LibraryCustomViewRule) Confidence() float64 { return api.ConfidenceMedium }

var hardcodedNsRe = regexp.MustCompile(`http://schemas\.android\.com/apk/res/[a-z][a-z0-9_.]+`)

// UnknownIDInLayoutRule reports a relative-layout style reference such as
// layout_below="@id/x" to an id that is declared in another layout but not in the
// layout holding the reference (AOSP UnknownIdInLayout).
type UnknownIDInLayoutRule struct {
	ResourceBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence. This is an
// Android-lint port from AOSP; the detection relies on source-text
// patterns (call names, string literal contents, hardcoded allow-
// lists of API names) rather than type resolution, so project-
// specific wrapper APIs can cause false positives or negatives.
// Classified per roadmap/17.
func (r *UnknownIDInLayoutRule) Confidence() float64 { return api.ConfidenceHigher }
