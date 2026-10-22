package rules

import (
	"bytes"
	"strings"

	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// ComposeSideEffectInCompositionRule flags direct assignments inside a
// @Composable function body that aren't wrapped in a recognized effect
// block (LaunchedEffect / SideEffect / DisposableEffect). Assignments
// during composition run on every recomposition and cause unpredictable
// behavior — mutations belong in an effect scope.
type ComposeSideEffectInCompositionRule struct {
	FlatDispatchBase
	BaseRule
	CustomPreviewWildcard bool
	CustomPreviewPrefixes []string
}

// Confidence reports a tier-2 (medium) base confidence. Compose rule. Detection relies on call-name token matching (e.g.
// 'verticalScroll', 'LazyColumn', 'rememberSaveable') rather than type
// resolution, so any project symbol with a matching name or token can
// produce a false match. Classified per roadmap/17.
func (r *ComposeSideEffectInCompositionRule) Confidence() float64 { return api.ConfidenceMedium }

var composePreviewAnnotationNames = []string{
	"Preview",
	"ComposePreview",
	"PreviewLightDark",
	"DarkPreview",
	"LightPreview",
	"NightPreview",
	"DayPreview",
	"DayNightPreviews",
}

func composeFileHasRuntimeComposableEvidence(file *scanner.File) bool {
	return fileImportsFQN(file, "androidx.compose.runtime.Composable") ||
		bytes.Contains(file.Content, []byte("@androidx.compose.runtime.Composable"))
}

// composeCodeOnlyContent returns a copy of file.Content with the bytes
// inside line comments, block comments, raw string bodies, and regular
// string bodies replaced with spaces. Byte offsets are preserved so
// callers may slice the result with the same indexes they use against
// file.Content. The evidence helpers in compose.go run text-level
// substring scans for patterns like `receiver: MutableState` and
// `val foo = remember {`; masking non-code bytes prevents comments or
// string literals from satisfying those scans.
func composeCodeOnlyContent(file *scanner.File) string {
	if file == nil || len(file.Content) == 0 {
		return ""
	}
	src := file.Content
	n := len(src)
	out := make([]byte, n)
	copy(out, src)
	for i := 0; i < n; {
		c := src[i]
		switch {
		case c == '/' && i+1 < n && src[i+1] == '/':
			i = composeMaskLineComment(src, out, i)
		case c == '/' && i+1 < n && src[i+1] == '*':
			i = composeMaskBlockComment(src, out, i)
		case c == '"' && i+2 < n && src[i+1] == '"' && src[i+2] == '"':
			i = composeMaskRawString(src, out, i)
		case c == '"':
			i = composeMaskLineString(src, out, i)
		default:
			i++
		}
	}
	return string(out)
}

// composeCodeOnlySlice returns the code-only view of the bytes between
// the start of fn and the start of assignment. The second return value
// is false when the range is invalid (assignment precedes fn or runs
// past file.Content).
func composeCodeOnlySlice(file *scanner.File, fn, assignment uint32) (string, bool) {
	if file == nil {
		return "", false
	}
	start := file.FlatStartByte(fn)
	end := file.FlatStartByte(assignment)
	if end <= start || int(end) > len(file.Content) {
		return "", false
	}
	masked := composeCodeOnlyContent(file)
	return masked[int(start):int(end)], true
}

func composeMaskLineComment(src, out []byte, i int) int {
	n := len(src)
	for i < n && src[i] != '\n' {
		out[i] = ' '
		i++
	}
	return i
}

func composeMaskBlockComment(src, out []byte, i int) int {
	n := len(src)
	out[i] = ' '
	out[i+1] = ' '
	i += 2
	for i+1 < n && (src[i] != '*' || src[i+1] != '/') {
		if src[i] != '\n' {
			out[i] = ' '
		}
		i++
	}
	if i+1 < n {
		out[i] = ' '
		out[i+1] = ' '
		return i + 2
	}
	for ; i < n; i++ {
		if src[i] != '\n' {
			out[i] = ' '
		}
	}
	return n
}

func composeMaskRawString(src, out []byte, i int) int {
	n := len(src)
	i += 3
	for i+2 < n && (src[i] != '"' || src[i+1] != '"' || src[i+2] != '"') {
		if src[i] != '\n' {
			out[i] = ' '
		}
		i++
	}
	if i+2 < n {
		return i + 3
	}
	for ; i < n; i++ {
		if src[i] != '\n' {
			out[i] = ' '
		}
	}
	return n
}

func composeMaskLineString(src, out []byte, i int) int {
	n := len(src)
	i++
	for i < n {
		c := src[i]
		if c == '\\' && i+1 < n {
			out[i] = ' '
			out[i+1] = ' '
			i += 2
			continue
		}
		if c == '"' {
			return i + 1
		}
		if c == '\n' {
			return i
		}
		out[i] = ' '
		i++
	}
	return n
}

func composeFunctionHasPreviewAnnotationWithConfig(file *scanner.File, fn uint32, cfg customPreviewConfig) bool {
	if flatHasCustomPreviewAnnotation(file, fn, cfg) {
		return true
	}
	for _, name := range composePreviewAnnotationNames {
		if flatHasAnnotationNamed(file, fn, name) {
			return true
		}
	}
	return false
}

func composeAssignmentIsMutableTransitionTargetState(file *scanner.File, assignment uint32, fn uint32) bool {
	if !fileImportsFQN(file, "androidx.compose.animation.core.MutableTransitionState") {
		return false
	}
	text := file.FlatNodeText(assignment)
	eq := strings.Index(text, "=")
	if eq < 0 {
		return false
	}
	lhs := strings.TrimSpace(text[:eq])
	if !strings.HasSuffix(lhs, ".targetState") {
		return false
	}
	receiver := strings.TrimSpace(strings.TrimSuffix(lhs, ".targetState"))
	receiver = strings.TrimPrefix(receiver, "this.")
	if receiver == "" || strings.ContainsAny(receiver, " \t\n(){}") {
		return false
	}
	prefix, ok := composeCodeOnlySlice(file, fn, assignment)
	if !ok {
		return false
	}
	return strings.Contains(prefix, "val "+receiver+" = remember { MutableTransitionState(") ||
		strings.Contains(prefix, "var "+receiver+" = remember { MutableTransitionState(")
}

func composeAssignmentSynchronizesRememberedObject(file *scanner.File, assignment uint32, fn uint32) bool {
	text := file.FlatNodeText(assignment)
	eq := strings.Index(text, "=")
	if eq < 0 {
		return false
	}
	lhs := strings.TrimSpace(text[:eq])
	dot := strings.Index(lhs, ".")
	if dot <= 0 {
		return false
	}
	receiver := strings.TrimSpace(strings.TrimPrefix(lhs[:dot], "this."))
	if receiver == "" || strings.ContainsAny(receiver, " \t\n(){}[]") {
		return false
	}
	prefix, ok := composeCodeOnlySlice(file, fn, assignment)
	if !ok {
		return false
	}
	return strings.Contains(prefix, "val "+receiver+" = remember {") ||
		strings.Contains(prefix, "var "+receiver+" = remember {") ||
		strings.Contains(prefix, "val "+receiver+" = remember(") ||
		strings.Contains(prefix, "var "+receiver+" = remember(")
}

func composeAssignmentCapturesComposableParameter(file *scanner.File, assignment uint32, fn uint32) bool {
	lhs, rhs, ok := composeAssignmentTextParts(file, assignment)
	if !ok {
		return false
	}
	lhs = strings.TrimPrefix(lhs, "this.")
	if lhs == "" || !strings.HasPrefix(lhs, "last") || strings.ContainsAny(lhs, ".[](){} \t\n") {
		return false
	}
	if rhs == "" || strings.ContainsAny(rhs, ".[](){} \t\n") {
		return false
	}
	for _, name := range flatFunctionParameterNames(file, fn) {
		if rhs == name {
			return true
		}
	}
	return false
}

func composeAssignmentRHSIsRememberOrAnimateCall(file *scanner.File, assignment uint32) bool {
	_, rhs, ok := composeAssignmentTextParts(file, assignment)
	if !ok {
		return false
	}
	name := composeLeadingCallName(rhs)
	return strings.HasPrefix(name, "remember") || strings.HasPrefix(name, "animate")
}

func composeAssignmentUpdatesMutableStateValue(file *scanner.File, assignment uint32, fn uint32) bool {
	lhs, _, ok := composeAssignmentTextParts(file, assignment)
	if !ok || !strings.HasSuffix(lhs, ".value") {
		return false
	}
	receiver := strings.TrimSpace(strings.TrimSuffix(lhs, ".value"))
	receiver = strings.TrimPrefix(receiver, "this.")
	if receiver == "" || strings.ContainsAny(receiver, ".[](){} \t\n") {
		return false
	}
	if composeReceiverHasMutableStateEvidence(file, receiver) {
		return true
	}
	name := flatFunctionName(file, fn)
	if !strings.HasPrefix(name, "Update") && !strings.HasPrefix(name, "Animate") {
		return false
	}
	return composeFileMentionsMutableStateAPI(file)
}

func composeReceiverHasMutableStateEvidence(file *scanner.File, receiver string) bool {
	if file == nil || receiver == "" {
		return false
	}
	text := composeCodeOnlyContent(file)
	for _, typ := range []string{
		"MutableState", "MutableFloatState", "MutableIntState", "MutableLongState", "MutableDoubleState",
	} {
		if strings.Contains(text, receiver+": "+typ) ||
			strings.Contains(text, receiver+": "+typ+"<") ||
			strings.Contains(text, receiver+" : "+typ) ||
			strings.Contains(text, receiver+" : "+typ+"<") {
			return true
		}
	}
	for _, factory := range []string{
		"mutableStateOf", "mutableFloatStateOf", "mutableIntStateOf", "mutableLongStateOf", "mutableDoubleStateOf",
	} {
		if strings.Contains(text, receiver+" = "+factory+"(") ||
			strings.Contains(text, receiver+" = "+factory+"<") ||
			strings.Contains(text, receiver+"="+factory+"(") ||
			strings.Contains(text, receiver+"="+factory+"<") ||
			strings.Contains(text, receiver+" = remember { "+factory+"(") ||
			strings.Contains(text, receiver+" = remember { "+factory+"<") ||
			strings.Contains(text, receiver+"=remember { "+factory+"(") ||
			strings.Contains(text, receiver+"=remember { "+factory+"<") ||
			strings.Contains(text, receiver+" = rememberSaveable { "+factory+"(") ||
			strings.Contains(text, receiver+" = rememberSaveable { "+factory+"<") ||
			strings.Contains(text, receiver+"=rememberSaveable { "+factory+"(") ||
			strings.Contains(text, receiver+"=rememberSaveable { "+factory+"<") {
			return true
		}
	}
	return false
}

func composeFileMentionsMutableStateAPI(file *scanner.File) bool {
	if file == nil {
		return false
	}
	return sourceImportsOrMentions(file, "androidx.compose.runtime.MutableState") ||
		sourceImportsOrMentions(file, "androidx.compose.runtime.MutableFloatState") ||
		sourceImportsOrMentions(file, "androidx.compose.runtime.MutableIntState") ||
		sourceImportsOrMentions(file, "androidx.compose.runtime.mutableStateOf") ||
		sourceImportsOrMentions(file, "androidx.compose.runtime.mutableFloatStateOf") ||
		sourceImportsOrMentions(file, "androidx.compose.runtime.mutableIntStateOf")
}

func composeAssignmentTextParts(file *scanner.File, assignment uint32) (string, string, bool) {
	if file == nil || assignment == 0 {
		return "", "", false
	}
	lhs, rhs, ok := strings.Cut(file.FlatNodeText(assignment), "=")
	if !ok {
		return "", "", false
	}
	return strings.TrimSpace(lhs), strings.TrimSpace(rhs), true
}

func composeLeadingCallName(expr string) string {
	expr = strings.TrimSpace(expr)
	if expr == "" {
		return ""
	}
	open := strings.Index(expr, "(")
	if open < 0 {
		return ""
	}
	prefix := strings.TrimSpace(expr[:open])
	if prefix == "" {
		return ""
	}
	if dot := strings.LastIndex(prefix, "."); dot >= 0 {
		prefix = prefix[dot+1:]
	}
	if prefix == "" {
		return ""
	}
	for _, r := range prefix {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' {
			continue
		}
		return ""
	}
	return prefix
}

func composeAssignmentIsDirectFunctionBodyStatement(file *scanner.File, assignment uint32, fn uint32) bool {
	parent, ok := file.FlatParent(assignment)
	if !ok {
		return false
	}
	switch file.FlatType(parent) {
	case "function_body":
		return flatNodeHasDirectParent(file, parent, fn)
	case "statements":
		body, ok := file.FlatParent(parent)
		return ok && file.FlatType(body) == "function_body" && flatNodeHasDirectParent(file, body, fn)
	default:
		return false
	}
}

func flatNodeHasDirectParent(file *scanner.File, node uint32, parent uint32) bool {
	actual, ok := file.FlatParent(node)
	return ok && actual == parent
}
