package rules

import (
	"regexp"
	"strings"

	"github.com/kaeawc/krit/internal/scanner"
)

type permissionAPI struct {
	api  string
	perm string
	re   *regexp.Regexp
}

var missingPermissionAPIs = []permissionAPI{
	{api: "requestLocationUpdates", perm: "ACCESS_FINE_LOCATION", re: regexp.MustCompile(`\brequestLocationUpdates\s*\(`)},
	{api: "getLastKnownLocation", perm: "ACCESS_FINE_LOCATION", re: regexp.MustCompile(`\bgetLastKnownLocation\s*\(`)},
	{api: "getCellLocation", perm: "ACCESS_COARSE_LOCATION", re: regexp.MustCompile(`\bgetCellLocation\s*\(`)},
	{api: "Camera.open", perm: "CAMERA", re: regexp.MustCompile(`\bCamera\.open\s*\(`)},
	{api: "setAudioSource", perm: "RECORD_AUDIO", re: regexp.MustCompile(`\bsetAudioSource\s*\(`)},
}

var permissionGuardTokens = []string{
	"checkSelfPermission",
	"ContextCompat.checkSelfPermission",
	"ActivityCompat.checkSelfPermission",
	"PermissionChecker.checkSelfPermission",
	"requestPermissions(",
	"ContextCompat.requestPermissions",
	"ActivityCompat.requestPermissions",
}

var leakyTagArgRe = regexp.MustCompile(`(?i)(^|[^A-Za-z0-9_])(view|activity|context|fragment|adapter|drawable|bitmap|cursor)([^A-Za-z0-9_]|$)`)

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
func (r *ViewConstructorRule) Confidence() float64 { return 0.75 }

func (r *ViewConstructorRule) NodeTypes() []string { return []string{"class_declaration"} }

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
	if body := file.FlatFindChild(idx, "class_body"); body != 0 {
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

func (r *ViewConstructorRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	if file.FlatHasModifier(idx, "abstract") {
		return nil
	}
	// Check only the class's own delegation_specifier children (not nested classes).
	// Extract the actual supertype name from the AST to avoid substring false positives
	// (e.g., "ViewModel" matching "View").
	isView := false
	for child := file.FlatFirstChild(idx); child != 0; child = file.FlatNextSib(child) {
		if file.FlatType(child) != "delegation_specifier" {
			continue
		}
		// delegation_specifier -> constructor_invocation -> user_type -> type_identifier
		typeName := viewConstructorSupertypeNameFlat(file, child)
		if typeName == "" {
			continue
		}
		for _, base := range viewSuperclasses {
			if typeName == base {
				isView = true
				break
			}
		}
		if isView {
			break
		}
	}
	if !isView {
		return nil
	}
	hasContextCtor, hasAttrSetCtor := false, false
	for child := file.FlatFirstChild(idx); child != 0; child = file.FlatNextSib(child) {
		if file.FlatType(child) == "primary_constructor" {
			ctorText := file.FlatNodeText(child)
			if strings.Contains(ctorText, "Context") {
				if strings.Contains(ctorText, "AttributeSet") {
					hasAttrSetCtor = true
				} else {
					hasContextCtor = true
				}
			}
		}
		if file.FlatType(child) == "class_body" {
			bodyText := file.FlatNodeText(child)
			if strings.Contains(bodyText, "constructor") {
				if strings.Contains(bodyText, "Context") && strings.Contains(bodyText, "AttributeSet") {
					hasAttrSetCtor = true
				}
				if strings.Contains(bodyText, "Context") {
					hasContextCtor = true
				}
			}
			if strings.Contains(file.FlatNodeText(idx), "@JvmOverloads") {
				hasContextCtor = true
				hasAttrSetCtor = true
			}
		}
	}
	if !hasContextCtor && !hasAttrSetCtor {
		return []scanner.Finding{r.Finding(file, file.FlatRow(idx)+1, 1,
			"Custom View subclass is missing (Context) and (Context, AttributeSet) constructors.")}
	}
	if !hasAttrSetCtor {
		return []scanner.Finding{r.Finding(file, file.FlatRow(idx)+1, 1,
			"Custom View subclass is missing (Context, AttributeSet) constructor needed for XML inflation.")}
	}
	return nil
}

func viewConstructorSupertypeNameFlat(file *scanner.File, deleg uint32) string {
	ut := file.FlatFindChild(deleg, "user_type")
	if ut == 0 {
		if ci := file.FlatFindChild(deleg, "constructor_invocation"); ci != 0 {
			ut = file.FlatFindChild(ci, "user_type")
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

type ViewTagRule struct {
	LineBase
	AndroidRule
}

var setTagRe = regexp.MustCompile(`\.setTag\s*\(`)
var setTagSingleArgRe = regexp.MustCompile(`\.setTag\s*\(\s*([A-Za-z_][A-Za-z0-9_\.]*)\s*\)`)

// Confidence reports a tier-2 (medium) base confidence. This is an
// Android-lint port from AOSP; the detection relies on source-text
// patterns (call names, string literal contents, hardcoded allow-
// lists of API names) rather than type resolution, so project-
// specific wrapper APIs can cause false positives or negatives.
// Classified per roadmap/17.
func (r *ViewTagRule) Confidence() float64 { return 0.75 }

func (r *ViewTagRule) CheckLines(file *scanner.File) []scanner.Finding {
	var findings []scanner.Finding
	for i, line := range file.Lines {
		if scanner.IsCommentLine(line) || !setTagRe.MatchString(line) {
			continue
		}
		loc := setTagRe.FindStringIndex(line)
		if loc == nil {
			continue
		}
		if enclosingAncestor(file, nodeAtPoint(file, i+1, loc[0]+1), "call_expression") == 0 {
			continue
		}
		matches := setTagSingleArgRe.FindStringSubmatch(line)
		if matches == nil || !leakyTagArgRe.MatchString(matches[1]) {
			continue
		}
		findings = append(findings, r.Finding(file, i+1, 1,
			"View.setTag() with a framework object may cause memory leaks across configuration changes."))
	}
	return findings
}

type WrongImportRule struct {
	LineBase
	AndroidRule
}

var wrongImportRe = regexp.MustCompile(`^import\s+android\.R\b`)

// Confidence reports a tier-2 (medium) base confidence. This is an
// Android-lint port from AOSP; the detection relies on source-text
// patterns (call names, string literal contents, hardcoded allow-
// lists of API names) rather than type resolution, so project-
// specific wrapper APIs can cause false positives or negatives.
// Classified per roadmap/17.
func (r *WrongImportRule) Confidence() float64 { return 0.75 }

func (r *WrongImportRule) CheckLines(file *scanner.File) []scanner.Finding {
	var findings []scanner.Finding
	for i, line := range file.Lines {
		if wrongImportRe.MatchString(strings.TrimSpace(line)) {
			findings = append(findings, r.Finding(file, i+1, 1,
				"Importing android.R instead of the application's R class. This may cause resource resolution errors."))
		}
	}
	return findings
}

type LayoutInflationRule struct {
	LineBase
	AndroidRule
}

var layoutInflateNullRe = regexp.MustCompile(`\.inflate\s*\(\s*R\.layout\.\w+\s*,\s*null\b`)

// layoutInflationDialogContexts are class suffixes / API markers where
// passing null to inflate() is the correct idiomatic pattern because no
// parent ViewGroup is available at inflation time.
var layoutInflationDialogContexts = []string{
	": Dialog", ": DialogFragment", ": BottomSheetDialogFragment",
	": AppCompatDialog", ": AppCompatDialogFragment", ": AlertDialog",
	": PopupWindow", ": ListPopupWindow",
	"MaterialAlertDialogBuilder", "AlertDialog.Builder",
	"onCreateDialog", "setView(", ".setContentView(",
	"PopupWindow(",
	// Compose interop via AndroidView — Compose manages attachment.
	"AndroidView(", "AndroidView {",
	// Offscreen rendering to a Bitmap/Canvas — no parent exists by design.
	"Bitmap.createBitmap", "createBitmap(",
	"Canvas(", "drawToBitmap",
}

// Confidence reports a tier-2 (medium) base confidence. This is an
// Android-lint port from AOSP; the detection relies on source-text
// patterns (call names, string literal contents, hardcoded allow-
// lists of API names) rather than type resolution, so project-
// specific wrapper APIs can cause false positives or negatives.
// Classified per roadmap/17.
func (r *LayoutInflationRule) Confidence() float64 { return 0.75 }

func (r *LayoutInflationRule) CheckLines(file *scanner.File) []scanner.Finding {
	var findings []scanner.Finding
	for i, line := range file.Lines {
		if scanner.IsCommentLine(line) || !layoutInflateNullRe.MatchString(line) {
			continue
		}
		loc := layoutInflateNullRe.FindStringIndex(line)
		if loc == nil {
			continue
		}
		if enclosingAncestor(file, nodeAtPoint(file, i+1, loc[0]+1), "call_expression") == 0 {
			continue
		}
		if containsAny(line, layoutInflationDialogContexts) {
			continue
		}
		prefix, ok := prefixTextBeforeLine(file, i+1, "function_declaration", "secondary_constructor", "anonymous_initializer", "lambda_literal")
		if ok {
			if containsAny(prefix, layoutInflationDialogContexts) {
				continue
			}
		} else {
			scopeNode := enclosingAncestor(file, nodeAtLine(file, i+1), "class_declaration", "object_declaration")
			if scopeNode == 0 {
				scopeNode = enclosingAncestor(file, nodeAtLine(file, i+1), "source_file")
			}
			if scopeNode == 0 {
				continue
			}
			if containsAny(declarationHeaderText(file, scopeNode), layoutInflationDialogContexts) {
				continue
			}
		}
		findings = append(findings, r.Finding(file, i+1, 1,
			"Avoid passing null as the parent ViewGroup. Inflate with the parent to get correct LayoutParams."))
	}
	return findings
}

type TrulyRandomRule struct {
	LineBase
	AndroidRule
}

var secureRandomSeedRe = regexp.MustCompile(`SecureRandom\s*\(\s*(byteArrayOf|"[^"]*"|ByteArray)`)

// Confidence reports a tier-2 (medium) base confidence. This is an
// Android-lint port from AOSP; the detection relies on source-text
// patterns (call names, string literal contents, hardcoded allow-
// lists of API names) rather than type resolution, so project-
// specific wrapper APIs can cause false positives or negatives.
// Classified per roadmap/17.
func (r *TrulyRandomRule) Confidence() float64 { return 0.75 }

func (r *TrulyRandomRule) CheckLines(file *scanner.File) []scanner.Finding {
	var findings []scanner.Finding
	for i, line := range file.Lines {
		if !scanner.IsCommentLine(line) && secureRandomSeedRe.MatchString(line) {
			findings = append(findings, r.Finding(file, i+1, 1,
				"SecureRandom with a hardcoded seed is not secure. Use the default constructor for cryptographic randomness."))
		}
	}
	return findings
}

type MissingPermissionRule struct {
	LineBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence. This is an
// Android-lint port from AOSP; the detection relies on source-text
// patterns (call names, string literal contents, hardcoded allow-
// lists of API names) rather than type resolution, so project-
// specific wrapper APIs can cause false positives or negatives.
// Classified per roadmap/17.
func (r *MissingPermissionRule) Confidence() float64 { return 0.75 }

func (r *MissingPermissionRule) CheckLines(file *scanner.File) []scanner.Finding {
	var findings []scanner.Finding
	for i, line := range file.Lines {
		trimmed := strings.TrimSpace(line)
		if scanner.IsCommentLine(line) || strings.HasPrefix(trimmed, "import ") {
			continue
		}
		var match *permissionAPI
		var loc []int
		for idx := range missingPermissionAPIs {
			api := &missingPermissionAPIs[idx]
			if candidate := api.re.FindStringIndex(line); candidate != nil {
				match = api
				loc = candidate
				break
			}
		}
		if match == nil {
			continue
		}
		if enclosingAncestor(file, nodeAtPoint(file, i+1, loc[0]), "call_expression") == 0 {
			continue
		}
		if containsAny(line, permissionGuardTokens) {
			continue
		}
		prefix, ok := prefixTextBeforeLine(file, i+1, "function_declaration", "secondary_constructor", "anonymous_initializer", "lambda_literal")
		if !ok {
			continue
		}
		if containsAny(prefix, permissionGuardTokens) {
			continue
		}
		findings = append(findings, r.Finding(file, i+1, 1,
			match.api+" requires "+match.perm+" permission. Ensure checkSelfPermission() is called before this API."))
	}
	return findings
}

type WrongConstantRule struct {
	LineBase
	AndroidRule
}
type wrongConstantEntry struct {
	Pattern     *regexp.Regexp
	Description string
}

var wrongConstantEntries = []wrongConstantEntry{
	{regexp.MustCompile(`\.setVisibility\s*\(\s*\d+\s*\)`), "setVisibility() should use View.VISIBLE, View.INVISIBLE, or View.GONE constants."},
	{regexp.MustCompile(`\.setLayoutDirection\s*\(\s*\d+\s*\)`), "setLayoutDirection() should use View.LAYOUT_DIRECTION_LTR or View.LAYOUT_DIRECTION_RTL constants."},
	{regexp.MustCompile(`\.setImportantForAccessibility\s*\(\s*\d+\s*\)`), "setImportantForAccessibility() should use View.IMPORTANT_FOR_ACCESSIBILITY_* constants."},
	{regexp.MustCompile(`\.setGravity\s*\(\s*\d+\s*\)`), "setGravity() should use Gravity.* constants."},
	{regexp.MustCompile(`\.setOrientation\s*\(\s*\d+\s*\)`), "setOrientation() should use LinearLayout.HORIZONTAL or LinearLayout.VERTICAL."},
}

// Confidence reports a tier-2 (medium) base confidence. This is an
// Android-lint port from AOSP; the detection relies on source-text
// patterns (call names, string literal contents, hardcoded allow-
// lists of API names) rather than type resolution, so project-
// specific wrapper APIs can cause false positives or negatives.
// Classified per roadmap/17.
func (r *WrongConstantRule) Confidence() float64 { return 0.75 }

func (r *WrongConstantRule) CheckLines(file *scanner.File) []scanner.Finding {
	var findings []scanner.Finding
	for i, line := range file.Lines {
		if scanner.IsCommentLine(line) {
			continue
		}
		for _, entry := range wrongConstantEntries {
			if entry.Pattern.MatchString(line) {
				findings = append(findings, r.Finding(file, i+1, 1, entry.Description))
				break
			}
		}
	}
	return findings
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
func (r *InstantiatableRule) Confidence() float64 { return 0.75 }

func (r *InstantiatableRule) NodeTypes() []string { return []string{"class_declaration"} }

var componentSuperclasses = []string{"Activity", "AppCompatActivity", "ComponentActivity", "FragmentActivity", "Service", "IntentService", "BroadcastReceiver", "ContentProvider", "Application"}

func (r *InstantiatableRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	text := file.FlatNodeText(idx)
	isComponent := false
	for _, base := range componentSuperclasses {
		if strings.Contains(text, ": "+base+"(") || strings.Contains(text, ": "+base+" ") ||
			strings.Contains(text, ": "+base+",") || strings.Contains(text, ": "+base+"{") {
			isComponent = true
			break
		}
	}
	if !isComponent {
		return nil
	}
	hasPrivateCtor := false
	for child := file.FlatFirstChild(idx); child != 0; child = file.FlatNextSib(child) {
		if file.FlatType(child) == "primary_constructor" {
			ctorText := file.FlatNodeText(child)
			if strings.Contains(ctorText, "private constructor") || strings.Contains(ctorText, "private ") {
				hasPrivateCtor = true
			}
		}
	}
	if strings.Contains(text, "private class ") {
		hasPrivateCtor = true
	}
	if hasPrivateCtor {
		return []scanner.Finding{r.Finding(file, file.FlatRow(idx)+1, 1,
			"This class is registered as an Android component but cannot be instantiated. Remove the private constructor or add a public no-arg constructor.")}
	}
	return nil
}

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
func (r *RtlAwareRule) Confidence() float64 { return 0.75 }

func (r *RtlAwareRule) NodeTypes() []string { return []string{"call_expression"} }

var rtlAwareMethods = map[string]string{".getLeft()": ".getStart()", ".getRight()": ".getEnd()", ".getPaddingLeft()": ".getPaddingStart()", ".getPaddingRight()": ".getPaddingEnd()", ".getLeft(": ".getStart(", ".getRight(": ".getEnd(", ".getPaddingLeft(": ".getPaddingStart(", ".getPaddingRight(": ".getPaddingEnd("}

func (r *RtlAwareRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	text := file.FlatNodeText(idx)
	for old, repl := range rtlAwareMethods {
		if strings.Contains(text, old) {
			return []scanner.Finding{r.Finding(file, file.FlatRow(idx)+1, 1,
				"Use RTL-aware "+repl+" instead of "+old+" for bidirectional layout support.")}
		}
	}
	return nil
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
func (r *RtlFieldAccessRule) Confidence() float64 { return 0.75 }

func (r *RtlFieldAccessRule) NodeTypes() []string { return []string{"string_literal"} }

var rtlFieldNames = []string{"mLeft", "mRight", "mPaddingLeft", "mPaddingRight"}

func (r *RtlFieldAccessRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	text := file.FlatNodeText(idx)
	for _, field := range rtlFieldNames {
		if text == "\""+field+"\"" {
			return []scanner.Finding{r.Finding(file, file.FlatRow(idx)+1, 1,
				"Direct access to View."+field+" via reflection is not RTL-aware. Use the corresponding getter method instead.")}
		}
	}
	return nil
}

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
func (r *GridLayoutRule) Confidence() float64 { return 0.75 }

func (r *GridLayoutRule) NodeTypes() []string { return []string{"call_expression"} }

var gridLayoutCreateRe = regexp.MustCompile(`GridLayout\s*\(`)

func (r *GridLayoutRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	text := file.FlatNodeText(idx)
	if !gridLayoutCreateRe.MatchString(text) {
		return nil
	}
	// Check surrounding context for columnCount
	startByte := int(file.FlatStartByte(idx))
	endByte := int(file.FlatEndByte(idx))
	contextStart := startByte - 200
	if contextStart < 0 {
		contextStart = 0
	}
	contextEnd := endByte + 200
	if contextEnd > len(file.Content) {
		contextEnd = len(file.Content)
	}
	context := string(file.Content[contextStart:contextEnd])
	if strings.Contains(context, "columnCount") {
		return nil
	}
	return []scanner.Finding{r.Finding(file, file.FlatRow(idx)+1, 1, "GridLayout should specify a columnCount. Without it, all children will be in a single row.")}
}

type LocaleFolderRule struct {
	LineBase
	AndroidRule
}

var localeFolderBadRe = regexp.MustCompile(`values-[a-z]{2}_[A-Z]{2}`)

// Confidence reports a tier-2 (medium) base confidence. This is an
// Android-lint port from AOSP; the detection relies on source-text
// patterns (call names, string literal contents, hardcoded allow-
// lists of API names) rather than type resolution, so project-
// specific wrapper APIs can cause false positives or negatives.
// Classified per roadmap/17.
func (r *LocaleFolderRule) Confidence() float64 { return 0.75 }

func (r *LocaleFolderRule) CheckLines(file *scanner.File) []scanner.Finding {
	var findings []scanner.Finding
	for i, line := range file.Lines {
		if scanner.IsCommentLine(line) {
			continue
		}
		if localeFolderBadRe.MatchString(line) {
			findings = append(findings, r.Finding(file, i+1, 1,
				"Wrong locale folder naming `"+localeFolderBadRe.FindString(line)+"`. Use `values-<lang>-r<REGION>` format (e.g., `values-en-rUS`)."))
		}
	}
	return findings
}

type UseAlpha2Rule struct {
	LineBase
	AndroidRule
}

var alpha3to2 = map[string]string{"eng": "en", "fra": "fr", "deu": "de", "spa": "es", "ita": "it", "por": "pt", "rus": "ru", "jpn": "ja", "kor": "ko", "zho": "zh", "ara": "ar", "hin": "hi", "tur": "tr", "pol": "pl", "nld": "nl", "swe": "sv", "nor": "no", "dan": "da", "fin": "fi", "tha": "th"}
var alpha3FolderRe = regexp.MustCompile(`values-([a-z]{3})\b`)

func (r *UseAlpha2Rule) CheckLines(file *scanner.File) []scanner.Finding {
	var findings []scanner.Finding
	for i, line := range file.Lines {
		if scanner.IsCommentLine(line) {
			continue
		}
		if m := alpha3FolderRe.FindStringSubmatch(line); m != nil {
			if repl, ok := alpha3to2[m[1]]; ok {
				findings = append(findings, r.Finding(file, i+1, 1,
					"Use 2-letter ISO 639-1 code `"+repl+"` instead of 3-letter code `"+m[1]+"` in locale folder."))
			}
		}
	}
	return findings
}

type MangledCRLFRule struct {
	LineBase
	AndroidRule
}

// Confidence bumps this line rule from the 0.75 line-rule default to
// 0.95 — mixed line endings is a literal substring check on the
// file bytes. Deterministic with no heuristic path.
func (r *MangledCRLFRule) Confidence() float64 { return 0.95 }

func (r *MangledCRLFRule) CheckLines(file *scanner.File) []scanner.Finding {
	content := string(file.Content)
	if strings.Contains(content, "\r\n") && strings.Contains(strings.ReplaceAll(content, "\r\n", ""), "\n") {
		return []scanner.Finding{r.Finding(file, 1, 1,
			"File has mixed line endings (both CRLF and LF). Use consistent line endings.")}
	}
	return nil
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
func (r *ResourceNameRule) Confidence() float64 { return 0.75 }

func (r *ResourceNameRule) NodeTypes() []string { return []string{"navigation_expression"} }

var resourceRefRe = regexp.MustCompile(`R\.(layout|drawable|string|color|dimen|style|menu|anim|xml|raw|id)\.([a-zA-Z_]\w*)`)
var snakeCaseRe = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)

func (r *ResourceNameRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	text := file.FlatNodeText(idx)
	for _, m := range resourceRefRe.FindAllStringSubmatch(text, -1) {
		if !snakeCaseRe.MatchString(m[2]) {
			return []scanner.Finding{r.Finding(file, file.FlatRow(idx)+1, 1, "Resource name `R."+m[1]+"."+m[2]+"` should use snake_case.")}
		}
	}
	return nil
}

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
func (r *ProguardRule) Confidence() float64 { return 0.75 }

func (r *ProguardRule) NodeTypes() []string { return []string{"string_literal"} }
func (r *ProguardRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	text := file.FlatNodeText(idx)
	if strings.Contains(text, "proguard.cfg") {
		return []scanner.Finding{r.Finding(file, file.FlatRow(idx)+1, 1, "Reference to obsolete `proguard.cfg`. Use `proguard-rules.pro` instead.")}
	}
	return nil
}

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
func (r *ProguardSplitRule) Confidence() float64 { return 0.75 }

func (r *ProguardSplitRule) CheckLines(file *scanner.File) []scanner.Finding {
	content := strings.Join(file.Lines, "\n")
	if !strings.Contains(content, "-keep") && !strings.Contains(content, "-dontwarn") {
		return nil
	}
	if proguardGenericRe.MatchString(content) && proguardSpecificRe.MatchString(content) {
		return []scanner.Finding{r.Finding(file, 1, 1, "Proguard configuration contains both generic Android rules and project-specific rules. Consider splitting into separate files for maintainability.")}
	}
	return nil
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
func (r *NfcTechWhitespaceRule) Confidence() float64 { return 0.75 }

func (r *NfcTechWhitespaceRule) NodeTypes() []string { return []string{"string_literal"} }

var nfcTechRe = regexp.MustCompile(`<tech>\s+\S+|<tech>\S+\s+</tech>`)

func (r *NfcTechWhitespaceRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	text := file.FlatNodeText(idx)
	if strings.Contains(text, "<tech>") && nfcTechRe.MatchString(text) {
		return []scanner.Finding{r.Finding(file, file.FlatRow(idx)+1, 1, "Whitespace in <tech> element value. NFC tech names must not have leading/trailing whitespace.")}
	}
	return nil
}

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
func (r *LibraryCustomViewRule) Confidence() float64 { return 0.75 }

func (r *LibraryCustomViewRule) NodeTypes() []string { return []string{"string_literal"} }

var hardcodedNsRe = regexp.MustCompile(`http://schemas\.android\.com/apk/res/[a-z][a-z0-9_.]+`)

func (r *LibraryCustomViewRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	text := file.FlatNodeText(idx)
	if hardcodedNsRe.MatchString(text) && !strings.Contains(text, "apk/res-auto") && !strings.Contains(text, "apk/res/android") {
		return []scanner.Finding{r.Finding(file, file.FlatRow(idx)+1, 1, "Use `http://schemas.android.com/apk/res-auto` instead of a hardcoded package namespace. Hardcoded namespaces don't work in library projects.")}
	}
	return nil
}

type UnknownIdInLayoutRule struct {
	FlatDispatchBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence. This is an
// Android-lint port from AOSP; the detection relies on source-text
// patterns (call names, string literal contents, hardcoded allow-
// lists of API names) rather than type resolution, so project-
// specific wrapper APIs can cause false positives or negatives.
// Classified per roadmap/17.
func (r *UnknownIdInLayoutRule) Confidence() float64 { return 0.75 }

func (r *UnknownIdInLayoutRule) NodeTypes() []string { return []string{"navigation_expression"} }

var idRefRe = regexp.MustCompile(`R\.id\.([a-zA-Z_]\w*)`)

func (r *UnknownIdInLayoutRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	text := file.FlatNodeText(idx)
	for _, m := range idRefRe.FindAllStringSubmatch(text, -1) {
		if strings.Contains(m[1], "__") || strings.HasPrefix(m[1], "_") {
			return []scanner.Finding{r.Finding(file, file.FlatRow(idx)+1, 1, "Suspicious ID reference `R.id."+m[1]+"`. Verify this ID exists in your layout resources.")}
		}
	}
	return nil
}
