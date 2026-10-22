package rules

import (
	"regexp"
	"strings"

	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

type ViewConstructorRule struct {
	FlatDispatchBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence. This is an
// Android-lint port from AOSP; the detection relies on source-text
// patterns (call names, string literal contents, hardcoded allow-
// lists of API names) rather than type resolution, so project-
// specific wrapper APIs can cause false positives or negatives.
// Classified per roadmap/17.
func (r *ViewConstructorRule) Confidence() float64 { return api.ConfidenceMedium }

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
	f := r.Finding(file, file.FlatRow(idx)+1, 1,
		"Importing android.R instead of the application's R class. This may cause resource resolution errors.")
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

var componentSuperclasses = []string{"Activity", "AppCompatActivity", "ComponentActivity", "FragmentActivity", "Service", "IntentService", "BroadcastReceiver", "ContentProvider", "Application"}

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
	"getLeft":         "getStart()",
	"getRight":        "getEnd()",
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

type ProguardRule struct {
	FlatDispatchBase
	AndroidRule
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
func (r *ProguardSplitRule) Confidence() float64 { return api.ConfidenceMedium }

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

type NfcTechWhitespaceRule struct {
	FlatDispatchBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence. This is an
// Android-lint port from AOSP; the detection relies on source-text
// patterns (call names, string literal contents, hardcoded allow-
// lists of API names) rather than type resolution, so project-
// specific wrapper APIs can cause false positives or negatives.
// Classified per roadmap/17.
func (r *NfcTechWhitespaceRule) Confidence() float64 { return api.ConfidenceMedium }

var nfcTechRe = regexp.MustCompile(`<tech>\s+\S+|<tech>\S+\s+</tech>`)

type LibraryCustomViewRule struct {
	FlatDispatchBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence. This is an
// Android-lint port from AOSP; the detection relies on source-text
// patterns (call names, string literal contents, hardcoded allow-
// lists of API names) rather than type resolution, so project-
// specific wrapper APIs can cause false positives or negatives.
// Classified per roadmap/17.
func (r *LibraryCustomViewRule) Confidence() float64 { return api.ConfidenceMedium }

var hardcodedNsRe = regexp.MustCompile(`http://schemas\.android\.com/apk/res/[a-z][a-z0-9_.]+`)

type UnknownIDInLayoutRule struct {
	FlatDispatchBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence. This is an
// Android-lint port from AOSP; the detection relies on source-text
// patterns (call names, string literal contents, hardcoded allow-
// lists of API names) rather than type resolution, so project-
// specific wrapper APIs can cause false positives or negatives.
// Classified per roadmap/17.
func (r *UnknownIDInLayoutRule) Confidence() float64 { return api.ConfidenceHigher }
