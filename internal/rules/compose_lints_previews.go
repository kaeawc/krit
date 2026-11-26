package rules

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"

	"github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// Messages of compose-lint-checks 1.6.1's preview detectors, as Lint reports them.
const (
	composePreviewPublicMessage = "Composables annotated with @Preview that are used only for previewing the UI should " +
		"not be public. See https://slackhq.github.io/compose-lints/rules/#preview-composables-should-not-be-public " +
		"for more information."
	composePreviewNamingMessageFormat = "Preview annotations with %d preview annotations should end with the `%s` " +
		"suffix. See https://slackhq.github.io/compose-lints/rules/#naming-multipreview-annotations-properly for more " +
		"information."
)

// composeLintsPreviewFQNs are the @Preview annotations (and the containers Kotlin uses for repeated ones).
var composeLintsPreviewFQNs = map[string]bool{
	"androidx.compose.ui.tooling.preview.Preview":                   true,
	"androidx.compose.ui.tooling.preview.Preview.Container":         true,
	"androidx.compose.desktop.ui.tooling.preview.Preview":           true,
	"androidx.compose.desktop.ui.tooling.preview.Preview.Container": true,
}

// composeLintsLibraryMultipreviews are library annotation classes annotated with @Preview.
var composeLintsLibraryMultipreviews = map[string]bool{
	"androidx.compose.ui.tooling.preview.PreviewDynamicColors":       true,
	"androidx.compose.ui.tooling.preview.PreviewFontScale":           true,
	"androidx.compose.ui.tooling.preview.PreviewLightDark":           true,
	"androidx.compose.ui.tooling.preview.PreviewScreenSizes":         true,
	"androidx.wear.compose.ui.tooling.preview.WearPreviewDevices":    true,
	"androidx.wear.compose.ui.tooling.preview.WearPreviewFontScales": true,
	"androidx.wear.compose.ui.tooling.preview.WearPreviewLargeRound": true,
	"androidx.wear.compose.ui.tooling.preview.WearPreviewSmallRound": true,
	"androidx.wear.compose.ui.tooling.preview.WearPreviewSquare":     true,
}

// composeLintsTestAnnotations exempt a public preview (compose-lints' TEST_ANNOTATIONS).
var composeLintsTestAnnotations = map[string]bool{
	"org.jetbrains.annotations.TestOnly":              true,
	"com.google.common.annotations.VisibleForTesting": true,
	"androidx.annotation.VisibleForTesting":           true,
}

// composeLintsPreviewMaxDepth is how many annotation-class hops compose-lints follows to find @Preview.
const composeLintsPreviewMaxDepth = 4

// ComposePreviewPublicRule ports compose-lints' ComposePreviewPublic: a public @Composable function annotated with
// @Preview, directly or through a multipreview annotation, should be private.
type ComposePreviewPublicRule struct {
	BaseRule
}

// ComposePreviewNamingRule ports compose-lints' ComposePreviewNaming: a multipreview annotation class carrying one
// preview annotation should end with `Preview`, and one carrying several should end with `Previews`.
type ComposePreviewNamingRule struct {
	BaseRule
}

// composeLintsAnnotationClass is a project annotation class declaration.
type composeLintsAnnotationClass struct {
	file *scanner.File
	decl uint32
}

// composeLintsPreviews resolves annotation entries to annotation classes across the project's Kotlin files and
// answers whether one is a (multi)preview. It is built once per cross-file run.
type composeLintsPreviews struct {
	classes map[string]composeLintsAnnotationClass
	memo    map[string]bool
}

func newComposeLintsPreviews(files []*scanner.File) *composeLintsPreviews {
	p := &composeLintsPreviews{classes: map[string]composeLintsAnnotationClass{}, memo: map[string]bool{}}
	for _, file := range files {
		if file == nil || file.FlatTree == nil || file.Language != scanner.LangKotlin ||
			!bytes.Contains(file.Content, []byte("annotation class")) {
			continue
		}
		pkg := filePackageName(file)
		for child := file.FlatFirstChild(0); child != 0; child = file.FlatNextSib(child) {
			if file.FlatType(child) != "class_declaration" || !composeLintsIsAnnotationClass(file, child) {
				continue
			}
			name := acrDeclarationName(file, child)
			fqn := name
			if pkg != "" {
				fqn = pkg + "." + name
			}
			p.classes[fqn] = composeLintsAnnotationClass{file: file, decl: child}
		}
	}
	return p
}

// resolve returns the FQN an annotation node names in file: an explicit import or alias, a fully qualified name,
// a same-package project class, or a wildcard import of a package holding a known class. "" when unknown.
func (p *composeLintsPreviews) resolve(file *scanner.File, annotation uint32) string {
	text := composeLintsAnnotationTypeText(file, annotation)
	if text == "" {
		return ""
	}
	first, rest, qualified := strings.Cut(text, ".")
	facts := fileFactsCache().Imports(file)
	if facts != nil {
		if fqn, ok := facts.Aliases[first]; ok {
			if qualified {
				return fqn + "." + rest
			}
			return fqn
		}
	}
	if qualified {
		if p.known(text) {
			return text
		}
		// `Preview.Container` style references keep their head's resolution below.
	}
	pkg := filePackageName(file)
	if candidate := composeLintsJoin(pkg, text); p.classes[candidate].file != nil {
		return candidate
	}
	if facts != nil {
		for wildcard := range facts.Wildcards {
			if candidate := strings.TrimSuffix(wildcard, "*") + text; p.known(candidate) {
				return candidate
			}
		}
	}
	return ""
}

func (p *composeLintsPreviews) known(fqn string) bool {
	return composeLintsPreviewFQNs[fqn] || composeLintsLibraryMultipreviews[fqn] || p.classes[fqn].file != nil
}

// isPreview reports whether an annotation class FQN is @Preview or carries it within the remaining depth, as
// compose-lints' checkIsPreview does.
func (p *composeLintsPreviews) isPreview(fqn string, depth int) bool {
	if depth >= composeLintsPreviewMaxDepth || fqn == "" {
		return false
	}
	if composeLintsPreviewFQNs[fqn] {
		return true
	}
	if composeLintsLibraryMultipreviews[fqn] {
		return depth+1 < composeLintsPreviewMaxDepth
	}
	key := fqn + "#" + strconv.Itoa(depth)
	if v, ok := p.memo[key]; ok {
		return v
	}
	p.memo[key] = false // break annotation cycles
	result := false
	if c := p.classes[fqn]; c.file != nil {
		result = p.annotatedIsPreview(c.file, c.decl, depth+1)
	}
	p.memo[key] = result
	return result
}

// annotatedIsPreview reports whether any annotation on decl resolves to a preview annotation class.
func (p *composeLintsPreviews) annotatedIsPreview(file *scanner.File, decl uint32, depth int) bool {
	if depth >= composeLintsPreviewMaxDepth {
		return false
	}
	found := false
	composeLintsEachAnnotation(file, decl, func(annotation uint32) {
		if !found && p.isPreview(p.resolve(file, annotation), depth) {
			found = true
		}
	})
	return found
}

// directPreviewCount counts the annotations on decl that are @Preview or an annotation class annotated with
// @Preview directly, which is how compose-lints sizes a multipreview.
func (p *composeLintsPreviews) directPreviewCount(file *scanner.File, decl uint32) int {
	count := 0
	composeLintsEachAnnotation(file, decl, func(annotation uint32) {
		fqn := p.resolve(file, annotation)
		switch {
		case composeLintsPreviewFQNs[fqn], composeLintsLibraryMultipreviews[fqn]:
			count++
		case p.classes[fqn].file != nil:
			c := p.classes[fqn]
			direct := false
			composeLintsEachAnnotation(c.file, c.decl, func(inner uint32) {
				if composeLintsPreviewFQNs[p.resolve(c.file, inner)] {
					direct = true
				}
			})
			if direct {
				count++
			}
		}
	})
	return count
}

func (r *ComposePreviewPublicRule) check(ctx *api.Context) {
	if ctx.CodeIndex == nil {
		return
	}
	previews := newComposeLintsPreviews(ctx.CodeIndex.Files)
	// A preview function's file names @Preview, a library multipreview (all named *Preview*), or a project one.
	markers := [][]byte{[]byte("Preview")}
	for fqn, c := range previews.classes {
		if name := fqn[strings.LastIndex(fqn, ".")+1:]; !strings.Contains(name, "Preview") &&
			previews.annotatedIsPreview(c.file, c.decl, 0) {
			markers = append(markers, []byte(name))
		}
	}
	for _, file := range ctx.CodeIndex.Files {
		if file == nil || file.FlatTree == nil || file.Language != scanner.LangKotlin ||
			!bytes.Contains(file.Content, []byte("Composable")) || !composeLintsContainsAny(file.Content, markers) {
			continue
		}
		file.FlatWalkNodes(0, "function_declaration", func(fn uint32) {
			if !acrFunctionIsComposable(file, fn) || acrIsLocalDeclaration(file, fn) || !composeLintsIsPublic(file, fn) ||
				!previews.annotatedIsPreview(file, fn, 0) || composeLintsHasTestAnnotation(file, fn) {
				return
			}
			line, col := composeLintsDeclarationStart(file, fn)
			ctx.Emit(r.Finding(file, line, col, composePreviewPublicMessage))
		})
	}
}

func (r *ComposePreviewNamingRule) check(ctx *api.Context) {
	if ctx.CodeIndex == nil {
		return
	}
	previews := newComposeLintsPreviews(ctx.CodeIndex.Files)
	for fqn, c := range previews.classes {
		if !previews.annotatedIsPreview(c.file, c.decl, 0) {
			continue
		}
		count := previews.directPreviewCount(c.file, c.decl)
		name := fqn[strings.LastIndex(fqn, ".")+1:]
		suffix := ""
		switch {
		case count == 1 && !strings.HasSuffix(name, "Preview"):
			suffix = "Preview"
		case count > 1 && !strings.HasSuffix(name, "Previews"):
			suffix = "Previews"
		default:
			continue
		}
		line, col := composeLintsDeclarationStart(c.file, c.decl)
		ctx.Emit(r.Finding(c.file, line, col, fmt.Sprintf(composePreviewNamingMessageFormat, count, suffix)))
	}
}

// composeLintsIsAnnotationClass reports whether a class declaration is an `annotation class`.
func composeLintsIsAnnotationClass(file *scanner.File, decl uint32) bool {
	mods, ok := file.FlatFindChild(decl, "modifiers")
	if !ok {
		return false
	}
	for m := file.FlatFirstChild(mods); m != 0; m = file.FlatNextSib(m) {
		if file.FlatType(m) == "class_modifier" && strings.TrimSpace(file.FlatNodeText(m)) == "annotation" {
			return true
		}
	}
	return false
}

// composeLintsEachAnnotation calls fn for each annotation in decl's modifiers.
func composeLintsEachAnnotation(file *scanner.File, decl uint32, fn func(annotation uint32)) {
	mods, ok := file.FlatFindChild(decl, "modifiers")
	if !ok {
		return
	}
	for m := file.FlatFirstChild(mods); m != 0; m = file.FlatNextSib(m) {
		if file.FlatType(m) == "annotation" {
			fn(m)
		}
	}
}

// composeLintsAnnotationTypeText returns the whitespace-free type an annotation names, without arguments or a
// use-site target, or "".
func composeLintsAnnotationTypeText(file *scanner.File, annotation uint32) string {
	for child := file.FlatFirstChild(annotation); child != 0; child = file.FlatNextSib(child) {
		switch file.FlatType(child) {
		case "use_site_target":
			return ""
		case "user_type":
			return composeLintsTypeText(file, child)
		case "constructor_invocation":
			if ut, ok := file.FlatFindChild(child, "user_type"); ok {
				return composeLintsTypeText(file, ut)
			}
			return ""
		}
	}
	return ""
}

// composeLintsTypeText returns a user_type's text without type arguments or whitespace.
func composeLintsTypeText(file *scanner.File, userType uint32) string {
	text := file.FlatNodeText(userType)
	if lt := strings.IndexByte(text, '<'); lt >= 0 {
		text = text[:lt]
	}
	return strings.Join(strings.Fields(text), "")
}

// composeLintsIsPublic reports whether a declaration has no visibility modifier or an explicit `public`.
func composeLintsIsPublic(file *scanner.File, decl uint32) bool {
	mods, ok := file.FlatFindChild(decl, "modifiers")
	if !ok {
		return true
	}
	for m := file.FlatFirstChild(mods); m != 0; m = file.FlatNextSib(m) {
		if file.FlatType(m) == "visibility_modifier" {
			return strings.TrimSpace(file.FlatNodeText(m)) == "public"
		}
	}
	return true
}

// composeLintsHasTestAnnotation reports whether decl carries @VisibleForTesting or @TestOnly.
func composeLintsHasTestAnnotation(file *scanner.File, decl uint32) bool {
	found := false
	composeLintsEachAnnotation(file, decl, func(annotation uint32) {
		text := composeLintsAnnotationTypeText(file, annotation)
		if composeLintsTestAnnotations[text] {
			found = true
			return
		}
		if facts := fileFactsCache().Imports(file); facts != nil && composeLintsTestAnnotations[facts.Aliases[text]] {
			found = true
		}
	})
	return found
}

func composeLintsJoin(pkg, name string) string {
	if pkg == "" {
		return name
	}
	return pkg + "." + name
}

func composeLintsContainsAny(content []byte, markers [][]byte) bool {
	for _, m := range markers {
		if bytes.Contains(content, m) {
			return true
		}
	}
	return false
}
