package rules

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/kaeawc/krit/internal/scanner"
)

type OverrideAbstractRule struct {
	FlatDispatchBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence. This is an
// Android-lint port from AOSP; the detection relies on source-text
// patterns (call names, string literal contents, hardcoded allow-
// lists of API names) rather than type resolution, so project-
// specific wrapper APIs can cause false positives or negatives.
// Classified per roadmap/17.
func (r *OverrideAbstractRule) Confidence() float64 { return 0.75 }

func (r *OverrideAbstractRule) NodeTypes() []string { return []string{"class_declaration"} }

var abstractClassRequirements = map[string][]string{"Service": {"onBind"}, "BroadcastReceiver": {"onReceive"}, "ContentProvider": {"onCreate", "query", "insert", "update", "delete", "getType"}}

func (r *OverrideAbstractRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	text := file.FlatNodeText(idx)
	var baseClass string
	var required []string
	for cls, reqs := range abstractClassRequirements {
		if strings.Contains(text, ": "+cls+"(") || strings.Contains(text, ": "+cls+" ") || strings.Contains(text, ": "+cls+",") || strings.Contains(text, ": "+cls+"{") || strings.Contains(text, ": "+cls+"()") {
			baseClass = cls
			required = reqs
			break
		}
	}
	if baseClass == "" || strings.Contains(text, "abstract class") {
		return nil
	}
	var missing []string
	for _, method := range required {
		if !strings.Contains(text, "override fun "+method+"(") && !strings.Contains(text, "override fun "+method+" (") {
			missing = append(missing, method)
		}
	}
	if len(missing) > 0 {
		return []scanner.Finding{r.Finding(file, file.FlatRow(idx)+1, 1, baseClass+" subclass must override: "+strings.Join(missing, ", ")+".")}
	}
	return nil
}


type ParcelCreatorRule struct {
	FlatDispatchBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence. This is an
// Android-lint port from AOSP; the detection relies on source-text
// patterns (call names, string literal contents, hardcoded allow-
// lists of API names) rather than type resolution, so project-
// specific wrapper APIs can cause false positives or negatives.
// Classified per roadmap/17.
func (r *ParcelCreatorRule) Confidence() float64 { return 0.75 }

func (r *ParcelCreatorRule) NodeTypes() []string { return []string{"class_declaration"} }

func (r *ParcelCreatorRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	text := file.FlatNodeText(idx)
	if !strings.Contains(text, "Parcelable") || strings.Contains(text, "@Parcelize") || strings.Contains(text, "Parcelize") || strings.Contains(text, "CREATOR") {
		return nil
	}
	return []scanner.Finding{r.Finding(file, file.FlatRow(idx)+1, 1, "Parcelable class missing CREATOR field. Use @Parcelize or add a CREATOR companion.")}
}


type SwitchIntDefRule struct {
	LineBase
	AndroidRule
}

var whenVisibilityRe = regexp.MustCompile(`when\s*\([^)]*visibility[^)]*\)\s*\{`)

// Confidence reports a tier-2 (medium) base confidence. This is an
// Android-lint port from AOSP; the detection relies on source-text
// patterns (call names, string literal contents, hardcoded allow-
// lists of API names) rather than type resolution, so project-
// specific wrapper APIs can cause false positives or negatives.
// Classified per roadmap/17.
func (r *SwitchIntDefRule) Confidence() float64 { return 0.75 }

func (r *SwitchIntDefRule) CheckLines(file *scanner.File) []scanner.Finding {
	var findings []scanner.Finding
	for i, line := range file.Lines {
		if !whenVisibilityRe.MatchString(line) {
			continue
		}
		hasVisible, hasInvisible, hasGone, hasElse := false, false, false, false
		depth := 0
		for j := i; j < len(file.Lines); j++ {
			l := file.Lines[j]
			depth += strings.Count(l, "{") - strings.Count(l, "}")
			if strings.Contains(l, "VISIBLE") && !strings.Contains(l, "INVISIBLE") {
				hasVisible = true
			}
			if strings.Contains(l, "INVISIBLE") {
				hasInvisible = true
			}
			if strings.Contains(l, "GONE") {
				hasGone = true
			}
			if strings.Contains(l, "else") {
				hasElse = true
			}
			if depth <= 0 && j > i {
				break
			}
		}
		if hasElse {
			continue
		}
		var missing []string
		if !hasVisible {
			missing = append(missing, "VISIBLE")
		}
		if !hasInvisible {
			missing = append(missing, "INVISIBLE")
		}
		if !hasGone {
			missing = append(missing, "GONE")
		}
		if len(missing) > 0 && len(missing) < 3 {
			findings = append(findings, r.Finding(file, i+1, 1, "when on visibility missing constants: "+strings.Join(missing, ", ")+". Add them or an else branch."))
		}
	}
	return findings
}


type TextViewEditsRule struct {
	LineBase
	AndroidRule
}

var textViewEditsRe = regexp.MustCompile(`\beditText\w*\.setText\b`)

// Confidence reports a tier-2 (medium) base confidence. This is an
// Android-lint port from AOSP; the detection relies on source-text
// patterns (call names, string literal contents, hardcoded allow-
// lists of API names) rather than type resolution, so project-
// specific wrapper APIs can cause false positives or negatives.
// Classified per roadmap/17.
func (r *TextViewEditsRule) Confidence() float64 { return 0.75 }

func (r *TextViewEditsRule) CheckLines(file *scanner.File) []scanner.Finding {
	var findings []scanner.Finding
	for i, line := range file.Lines {
		if textViewEditsRe.MatchString(line) {
			findings = append(findings, r.Finding(file, i+1, 1, "Using setText on an EditText. Consider using Editable or getText()."))
		}
	}
	return findings
}


type WrongViewCastRule struct {
	LineBase
	AndroidRule
}

var viewIdPrefixMap = map[string][]string{"btn_": {"Button", "MaterialButton", "AppCompatButton", "ImageButton", "ToggleButton", "RadioButton", "CompoundButton"}, "button_": {"Button", "MaterialButton", "AppCompatButton", "ImageButton", "ToggleButton", "RadioButton", "CompoundButton"}, "tv_": {"TextView", "AppCompatTextView", "MaterialTextView"}, "text_": {"TextView", "AppCompatTextView", "MaterialTextView"}, "iv_": {"ImageView", "AppCompatImageView", "ShapeableImageView"}, "img_": {"ImageView", "AppCompatImageView", "ShapeableImageView"}, "image_": {"ImageView", "AppCompatImageView", "ShapeableImageView"}, "rv_": {"RecyclerView"}, "recycler_": {"RecyclerView"}, "et_": {"EditText", "AppCompatEditText", "TextInputEditText"}, "edit_": {"EditText", "AppCompatEditText", "TextInputEditText"}, "input_": {"EditText", "AppCompatEditText", "TextInputEditText"}}

var (
	findViewByIdGenericRe = regexp.MustCompile(`findViewById<(\w+)>\s*\(\s*R\.id\.(\w+)\)`)
	findViewByIdCastRe    = regexp.MustCompile(`findViewById\s*\(\s*R\.id\.(\w+)\)\s+as\s+(\w+)`)
)

// Confidence reports a tier-2 (medium) base confidence. This is an
// Android-lint port from AOSP; the detection relies on source-text
// patterns (call names, string literal contents, hardcoded allow-
// lists of API names) rather than type resolution, so project-
// specific wrapper APIs can cause false positives or negatives.
// Classified per roadmap/17.
func (r *WrongViewCastRule) Confidence() float64 { return 0.75 }

func (r *WrongViewCastRule) CheckLines(file *scanner.File) []scanner.Finding {
	var findings []scanner.Finding
	for i, line := range file.Lines {
		if scanner.IsCommentLine(line) {
			continue
		}
		var castType, idName string
		if m := findViewByIdGenericRe.FindStringSubmatch(line); m != nil {
			castType = m[1]
			idName = m[2]
		} else if m := findViewByIdCastRe.FindStringSubmatch(line); m != nil {
			idName = m[1]
			castType = m[2]
		}
		if castType == "" || idName == "" {
			continue
		}
		idLower := strings.ToLower(idName)
		for prefix, expectedTypes := range viewIdPrefixMap {
			if strings.HasPrefix(idLower, prefix) {
				compatible := false
				for _, et := range expectedTypes {
					if castType == et {
						compatible = true
						break
					}
				}
				if !compatible {
					findings = append(findings, r.Finding(file, i+1, 1, "Suspicious cast: id '"+idName+"' (prefix '"+prefix+"') suggests "+expectedTypes[0]+", but cast to "+castType+"."))
				}
				break
			}
		}
	}
	return findings
}


type DeprecatedRule struct {
	LineBase
	AndroidRule
}
type deprecatedApiEntry struct {
	Pattern string
	Message string
}

var deprecatedApis = []deprecatedApiEntry{{"AsyncTask", "AsyncTask is deprecated as of API 30. Use java.util.concurrent or Kotlin coroutines instead."}, {"IntentService", "IntentService is deprecated as of API 30. Use WorkManager or JobIntentService instead."}, {"PreferenceActivity", "PreferenceActivity is deprecated as of API 29. Use PreferenceFragmentCompat instead."}, {"CursorLoader", "CursorLoader is deprecated as of API 28. Use Room with LiveData or Flow instead."}, {"LocalBroadcastManager", "LocalBroadcastManager is deprecated. Use LiveData, Flow, or other observable patterns instead."}, {"TabActivity", "TabActivity is deprecated as of API 13. Use tabs with Fragment/ViewPager instead."}, {"ActivityGroup", "ActivityGroup is deprecated as of API 13. Use Fragment-based navigation instead."}, {"getRunningTasks", "getRunningTasks is deprecated as of API 21. It returns only the caller's own tasks for privacy."}, {"DefaultHttpClient", "DefaultHttpClient is deprecated as of API 22. Use HttpURLConnection or OkHttp instead."}, {"AndroidHttpClient", "AndroidHttpClient is deprecated as of API 22. Use HttpURLConnection or OkHttp instead."}}

// Confidence reports a tier-2 (medium) base confidence. This is an
// Android-lint port from AOSP; the detection relies on source-text
// patterns (call names, string literal contents, hardcoded allow-
// lists of API names) rather than type resolution, so project-
// specific wrapper APIs can cause false positives or negatives.
// Classified per roadmap/17.
func (r *DeprecatedRule) Confidence() float64 { return 0.75 }

func (r *DeprecatedRule) CheckLines(file *scanner.File) []scanner.Finding {
	var findings []scanner.Finding
	for i, line := range file.Lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || scanner.IsCommentLine(line) || strings.HasPrefix(trimmed, "import ") {
			continue
		}
		for _, entry := range deprecatedApis {
			if strings.Contains(line, entry.Pattern) {
				findings = append(findings, r.Finding(file, i+1, 1, entry.Message))
				break
			}
		}
	}
	return findings
}

type RangeRule struct {
	LineBase
	AndroidRule
}

var (
	rangeSetAlphaRe    = regexp.MustCompile(`\.setAlpha\s*\(\s*(-?\d+)\s*\)`)
	rangeColorArgbRe   = regexp.MustCompile(`Color\.(argb|rgb)\s*\(([^)]+)\)`)
	rangeSetProgressRe = regexp.MustCompile(`\.setProgress\s*\(\s*(-?\d+)\s*\)`)
	rangeSetRotationRe = regexp.MustCompile(`\.setRotation\s*\(\s*(-?\d+(?:\.\d+)?)\s*\)`)
	rangeNumericRe     = regexp.MustCompile(`^-?\d+$`)
)

// Confidence reports a tier-2 (medium) base confidence. This is an
// Android-lint port from AOSP; the detection relies on source-text
// patterns (call names, string literal contents, hardcoded allow-
// lists of API names) rather than type resolution, so project-
// specific wrapper APIs can cause false positives or negatives.
// Classified per roadmap/17.
func (r *RangeRule) Confidence() float64 { return 0.75 }

func (r *RangeRule) CheckLines(file *scanner.File) []scanner.Finding {
	var findings []scanner.Finding
	for i, line := range file.Lines {
		if scanner.IsCommentLine(line) {
			continue
		}
		if m := rangeSetAlphaRe.FindStringSubmatch(line); m != nil {
			if v := parseInt(m[1]); v < 0 || v > 255 {
				findings = append(findings, r.Finding(file, i+1, 1, "setAlpha() value "+m[1]+" is outside the valid range [0, 255]."))
			}
		}
		if m := rangeColorArgbRe.FindStringSubmatch(line); m != nil {
			for _, arg := range strings.Split(m[2], ",") {
				arg = strings.TrimSpace(arg)
				if rangeNumericRe.MatchString(arg) {
					if v := parseInt(arg); v < 0 || v > 255 {
						findings = append(findings, r.Finding(file, i+1, 1, "Color."+m[1]+"() argument "+arg+" is outside the valid range [0, 255]."))
						break
					}
				}
			}
		}
		if m := rangeSetProgressRe.FindStringSubmatch(line); m != nil {
			if v := parseInt(m[1]); v < 0 || v > 100 {
				findings = append(findings, r.Finding(file, i+1, 1, "setProgress() value "+m[1]+" is outside the valid range [0, 100]."))
			}
		}
		if m := rangeSetRotationRe.FindStringSubmatch(line); m != nil {
			if v := parseFloat(m[1]); v < -360 || v > 360 {
				findings = append(findings, r.Finding(file, i+1, 1, "setRotation() value "+m[1]+" is outside the typical range [-360, 360]."))
			}
		}
	}
	return findings
}


type ResourceTypeRule struct {
	LineBase
	AndroidRule
}

var resourceMethodExpected = map[string]string{"getString": "string", "getText": "string", "getQuantityString": "plurals", "getStringArray": "array", "getIntArray": "array", "getDrawable": "drawable", "setImageResource": "drawable", "setImageDrawable": "drawable", "setContentView": "layout", "inflate": "layout", "getColor": "color", "getColorStateList": "color", "getDimension": "dimen", "getDimensionPixelSize": "dimen", "getDimensionPixelOffset": "dimen", "getBoolean": "bool", "getInteger": "integer", "getAnimation": "anim", "getLayout": "layout"}
var resourceCallRe = regexp.MustCompile(`\b(\w+)\s*\(\s*R\.(\w+)\.(\w+)`)

// Confidence reports a tier-2 (medium) base confidence. This is an
// Android-lint port from AOSP; the detection relies on source-text
// patterns (call names, string literal contents, hardcoded allow-
// lists of API names) rather than type resolution, so project-
// specific wrapper APIs can cause false positives or negatives.
// Classified per roadmap/17.
func (r *ResourceTypeRule) Confidence() float64 { return 0.75 }

func (r *ResourceTypeRule) CheckLines(file *scanner.File) []scanner.Finding {
	var findings []scanner.Finding
	for i, line := range file.Lines {
		if scanner.IsCommentLine(line) {
			continue
		}
		for _, m := range resourceCallRe.FindAllStringSubmatch(line, -1) {
			if expected, ok := resourceMethodExpected[m[1]]; ok && m[2] != expected {
				findings = append(findings, r.Finding(file, i+1, 1, fmt.Sprintf("%s(R.%s.%s): expected R.%s resource, not R.%s.", m[1], m[2], m[3], expected, m[2])))
			}
		}
	}
	return findings
}

type ResourceAsColorRule struct {
	LineBase
	AndroidRule
}

var resAsColorRe = regexp.MustCompile(`\.(setBackgroundColor|setTextColor|setColor)\s*\(\s*R\.`)

// Confidence reports a tier-2 (medium) base confidence. This is an
// Android-lint port from AOSP; the detection relies on source-text
// patterns (call names, string literal contents, hardcoded allow-
// lists of API names) rather than type resolution, so project-
// specific wrapper APIs can cause false positives or negatives.
// Classified per roadmap/17.
func (r *ResourceAsColorRule) Confidence() float64 { return 0.75 }

func (r *ResourceAsColorRule) CheckLines(file *scanner.File) []scanner.Finding {
	var findings []scanner.Finding
	for i, line := range file.Lines {
		if resAsColorRe.MatchString(line) {
			findings = append(findings, r.Finding(file, i+1, 1, "Passing a resource ID where a color value is expected. Use ContextCompat.getColor() instead."))
		}
	}
	return findings
}


type SupportAnnotationUsageRule struct {
	LineBase
	AndroidRule
}

var mainThreadAnnotRe = regexp.MustCompile(`@MainThread`)
var ioMethodPatterns = []string{"HttpURLConnection", "OkHttpClient", "FileInputStream", "FileOutputStream", "BufferedReader", "BufferedWriter", "URLConnection", "HttpClient", "Retrofit", "Socket(", "ServerSocket(", "DatagramSocket(", "FileReader", "FileWriter", "RandomAccessFile", "openConnection("}

// Confidence reports a tier-2 (medium) base confidence. This is an
// Android-lint port from AOSP; the detection relies on source-text
// patterns (call names, string literal contents, hardcoded allow-
// lists of API names) rather than type resolution, so project-
// specific wrapper APIs can cause false positives or negatives.
// Classified per roadmap/17.
func (r *SupportAnnotationUsageRule) Confidence() float64 { return 0.75 }

func (r *SupportAnnotationUsageRule) CheckLines(file *scanner.File) []scanner.Finding {
	var findings []scanner.Finding
	inMainThreadFun := false
	mainThreadLine := 0
	braceDepth := 0
	for i, line := range file.Lines {
		trimmed := strings.TrimSpace(line)
		if mainThreadAnnotRe.MatchString(trimmed) {
			inMainThreadFun = true
			mainThreadLine = i + 1
			continue
		}
		if inMainThreadFun && braceDepth == 0 {
			if strings.Contains(line, "{") {
				braceDepth = 1
			}
			if !strings.Contains(line, "fun ") && !strings.Contains(line, "{") && !strings.HasPrefix(trimmed, "@") && trimmed != "" {
				inMainThreadFun = false
			}
			continue
		}
		if inMainThreadFun && braceDepth > 0 {
			braceDepth += strings.Count(line, "{") - strings.Count(line, "}")
			if braceDepth <= 0 {
				inMainThreadFun = false
				braceDepth = 0
				continue
			}
			for _, pat := range ioMethodPatterns {
				if strings.Contains(line, pat) {
					findings = append(findings, r.Finding(file, i+1, 1, fmt.Sprintf("@MainThread function (line %d) performs IO/network operation (%s). This may block the UI thread.", mainThreadLine, pat)))
					break
				}
			}
		}
	}
	return findings
}

type AccidentalOctalRule struct {
	LineBase
	AndroidRule
}

var accidentalOctalRe = regexp.MustCompile(`\b0\d{2,}\b`)

// Confidence reports a tier-2 (medium) base confidence. This is an
// Android-lint port from AOSP; the detection relies on source-text
// patterns (call names, string literal contents, hardcoded allow-
// lists of API names) rather than type resolution, so project-
// specific wrapper APIs can cause false positives or negatives.
// Classified per roadmap/17.
func (r *AccidentalOctalRule) Confidence() float64 { return 0.75 }

func (r *AccidentalOctalRule) CheckLines(file *scanner.File) []scanner.Finding {
	var findings []scanner.Finding
	for i, line := range file.Lines {
		if !scanner.IsCommentLine(line) && accidentalOctalRe.MatchString(line) && !strings.Contains(line, "0x") && !strings.Contains(line, "0b") {
			findings = append(findings, r.Finding(file, i+1, 1, "Suspicious leading zero \u2014 this may be an accidental octal literal."))
		}
	}
	return findings
}


type AppCompatMethodRule struct {
	LineBase
	AndroidRule
}

var appCompatMethodRe = regexp.MustCompile(`\b(getActionBar|setProgressBarVisibility|setProgressBarIndeterminateVisibility)\s*\(`)

// Confidence reports a tier-2 (medium) base confidence. This is an
// Android-lint port from AOSP; the detection relies on source-text
// patterns (call names, string literal contents, hardcoded allow-
// lists of API names) rather than type resolution, so project-
// specific wrapper APIs can cause false positives or negatives.
// Classified per roadmap/17.
func (r *AppCompatMethodRule) Confidence() float64 { return 0.75 }

func (r *AppCompatMethodRule) CheckLines(file *scanner.File) []scanner.Finding {
	var findings []scanner.Finding
	for i, line := range file.Lines {
		if appCompatMethodRe.MatchString(line) && !strings.Contains(line, "getSupportActionBar") {
			findings = append(findings, r.Finding(file, i+1, 1, "Use AppCompat equivalent methods for backward compatibility."))
		}
	}
	return findings
}


type CustomViewStyleableRule struct {
	FlatDispatchBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence. This is an
// Android-lint port from AOSP; the detection relies on source-text
// patterns (call names, string literal contents, hardcoded allow-
// lists of API names) rather than type resolution, so project-
// specific wrapper APIs can cause false positives or negatives.
// Classified per roadmap/17.
func (r *CustomViewStyleableRule) Confidence() float64 { return 0.75 }

func (r *CustomViewStyleableRule) NodeTypes() []string { return []string{"call_expression"} }

var obtainStyledAttrsRe = regexp.MustCompile(`obtainStyledAttributes\s*\(\s*\w+\s*,\s*R\.styleable\.(\w+)`)

func (r *CustomViewStyleableRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	text := file.FlatNodeText(idx)
	m := obtainStyledAttrsRe.FindStringSubmatch(text)
	if m == nil {
		return nil
	}
	// Walk up to find enclosing class name
	var className string
	for parent, ok := file.FlatParent(idx); ok; parent, ok = file.FlatParent(parent) {
		if file.FlatType(parent) == "class_declaration" {
			classText := file.FlatNodeText(parent)
			if cm := classNameRe.FindStringSubmatch(classText); cm != nil {
				className = cm[1]
			}
			break
		}
	}
	if className == "" || m[1] == className {
		return nil
	}
	return []scanner.Finding{r.Finding(file, file.FlatRow(idx)+1, 1,
		fmt.Sprintf("Custom view '%s' uses R.styleable.%s \u2014 expected R.styleable.%s to match the class name.", className, m[1], className))}
}

type DalvikOverrideRule struct {
	FlatDispatchBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence. This is an
// Android-lint port from AOSP; the detection relies on source-text
// patterns (call names, string literal contents, hardcoded allow-
// lists of API names) rather than type resolution, so project-
// specific wrapper APIs can cause false positives or negatives.
// Classified per roadmap/17.
func (r *DalvikOverrideRule) Confidence() float64 { return 0.75 }


type InnerclassSeparatorRule struct {
	LineBase
	AndroidRule
}

var innerclassSepRe = regexp.MustCompile(`"[^"]*\w/\w+\w"`)

// Confidence reports a tier-2 (medium) base confidence. This is an
// Android-lint port from AOSP; the detection relies on source-text
// patterns (call names, string literal contents, hardcoded allow-
// lists of API names) rather than type resolution, so project-
// specific wrapper APIs can cause false positives or negatives.
// Classified per roadmap/17.
func (r *InnerclassSeparatorRule) Confidence() float64 { return 0.75 }

func (r *InnerclassSeparatorRule) CheckLines(file *scanner.File) []scanner.Finding {
	var findings []scanner.Finding
	for i, line := range file.Lines {
		if strings.Contains(line, "Class.forName") && innerclassSepRe.MatchString(line) && !strings.Contains(line, "$") {
			findings = append(findings, r.Finding(file, i+1, 1, "Use '$' instead of '/' as inner class separator in class names."))
		}
	}
	return findings
}


type ObjectAnimatorBindingRule struct {
	LineBase
	AndroidRule
}

var objAnimatorRe = regexp.MustCompile(`ObjectAnimator\.of(?:Float|Int|Object)\s*\([^,]+,\s*"([^"]+)"`)
var knownAnimatorProperties = map[string]bool{"alpha": true, "translationX": true, "translationY": true, "translationZ": true, "rotation": true, "rotationX": true, "rotationY": true, "scaleX": true, "scaleY": true, "x": true, "y": true, "z": true, "elevation": true}

// Confidence reports a tier-2 (medium) base confidence. This is an
// Android-lint port from AOSP; the detection relies on source-text
// patterns (call names, string literal contents, hardcoded allow-
// lists of API names) rather than type resolution, so project-
// specific wrapper APIs can cause false positives or negatives.
// Classified per roadmap/17.
func (r *ObjectAnimatorBindingRule) Confidence() float64 { return 0.75 }

func (r *ObjectAnimatorBindingRule) CheckLines(file *scanner.File) []scanner.Finding {
	var findings []scanner.Finding
	for i, line := range file.Lines {
		if !scanner.IsCommentLine(line) {
			if m := objAnimatorRe.FindStringSubmatch(line); m != nil && !knownAnimatorProperties[m[1]] {
				findings = append(findings, r.Finding(file, i+1, 1, "ObjectAnimator property \""+m[1]+"\" is not a standard View property. Verify the target has a setter for this property."))
			}
		}
	}
	return findings
}


type OnClickRule struct {
	FlatDispatchBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence. This is an
// Android-lint port from AOSP; the detection relies on source-text
// patterns (call names, string literal contents, hardcoded allow-
// lists of API names) rather than type resolution, so project-
// specific wrapper APIs can cause false positives or negatives.
// Classified per roadmap/17.
func (r *OnClickRule) Confidence() float64 { return 0.75 }


type PropertyEscapeRule struct {
	LineBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence. This is an
// Android-lint port from AOSP; the detection relies on source-text
// patterns (call names, string literal contents, hardcoded allow-
// lists of API names) rather than type resolution, so project-
// specific wrapper APIs can cause false positives or negatives.
// Classified per roadmap/17.
func (r *PropertyEscapeRule) Confidence() float64 { return 0.75 }

func (r *PropertyEscapeRule) CheckLines(file *scanner.File) []scanner.Finding {
	var findings []scanner.Finding
	inMultilineString := false
	for i, line := range file.Lines {
		if scanner.IsCommentLine(line) {
			continue
		}
		if strings.Count(line, `"""`)%2 != 0 {
			inMultilineString = !inMultilineString
		}
		if inMultilineString {
			continue
		}
		inString := false
		for j := 0; j < len(line); j++ {
			ch := line[j]
			if ch == '"' && (j == 0 || line[j-1] != '\\') {
				inString = !inString
				continue
			}
			if inString && ch == '\\' && j+1 < len(line) {
				next := line[j+1]
				switch next {
				case 'n', 't', 'r', '\\', '"', '\'', '$', 'b', 'u', 'f':
					j++
				default:
					if next >= '0' && next <= '9' {
						j++
						continue
					}
					findings = append(findings, r.Finding(file, i+1, 1, "Invalid escape sequence '\\"+string(next)+"' in string literal."))
					j++
				}
			}
		}
	}
	return findings
}

type ShortAlarmRule struct {
	LineBase
	AndroidRule
}

var shortAlarmRe = regexp.MustCompile(`\b(setRepeating|setInexactRepeating)\s*\(`)

// Confidence reports a tier-2 (medium) base confidence. This is an
// Android-lint port from AOSP; the detection relies on source-text
// patterns (call names, string literal contents, hardcoded allow-
// lists of API names) rather than type resolution, so project-
// specific wrapper APIs can cause false positives or negatives.
// Classified per roadmap/17.
func (r *ShortAlarmRule) Confidence() float64 { return 0.75 }

func (r *ShortAlarmRule) CheckLines(file *scanner.File) []scanner.Finding {
	var findings []scanner.Finding
	for i, line := range file.Lines {
		if shortAlarmRe.MatchString(line) && (strings.Contains(line, "1000") || strings.Contains(line, "5000") || strings.Contains(line, "10000") || strings.Contains(line, "30000")) {
			findings = append(findings, r.Finding(file, i+1, 1, "Short alarm interval. Consider using a minimum of 60 seconds for repeating alarms."))
		}
	}
	return findings
}


type LocalSuppressRule struct {
	LineBase
	AndroidRule
}

var suppressLintRe = regexp.MustCompile(`@SuppressLint\s*\(\s*"([^"]+)"`)
var knownLintIssueIDs = map[string]bool{"NewApi": true, "InlinedApi": true, "Override": true, "UnusedResources": true, "HardcodedText": true, "MissingTranslation": true, "ExtraTranslation": true, "MissingPermission": true, "ContentDescription": true, "ObsoleteLayoutParam": true, "ViewHolder": true, "LogConditional": true, "SdCardPath": true, "Wakelock": true, "SetJavaScriptEnabled": true, "ExportedService": true, "PackagedPrivateKey": true, "ValidFragment": true, "ViewConstructor": true, "WrongImport": true, "ServiceCast": true, "LayoutInflation": true, "ShowToast": true, "PackageManagerGetSignatures": true, "UseSparseArrays": true, "UseValueOf": true, "LongLogTag": true, "LogTagMismatch": true, "UnlocalizedSms": true, "ViewTag": true, "ShortAlarm": true, "UniqueConstants": true, "ShiftFlags": true, "AccidentalOctal": true, "AppCompatMethod": true, "CheckResult": true, "CommitPrefEdits": true, "CommitTransaction": true, "CustomViewStyleable": true, "CutPasteId": true, "DalvikOverride": true, "DefaultLocale": true, "Deprecated": true, "DeviceAdmin": true, "DuplicateActivity": true, "DuplicateIds": true, "DuplicateIncludedIds": true, "DuplicateUsesFeature": true, "ExtraText": true, "FullBackupContent": true, "GradleCompatible": true, "GradleDependency": true, "GradleDeprecated": true, "GradleDynamicVersion": true, "GradleGetter": true, "GradleIdeError": true, "GradleOverrides": true, "GradlePath": true, "IllegalResourceRef": true, "InconsistentArrays": true, "InconsistentLayout": true, "InnerclassSeparator": true, "InvalidResourceFolder": true, "ManifestOrder": true, "ManifestTypo": true, "MissingApplicationIcon": true, "MissingId": true, "MissingRegistered": true, "MissingVersion": true, "MockLocation": true, "MultipleUsesSdk": true, "NestedScrolling": true, "NotSibling": true, "ObjectAnimatorBinding": true, "OnClick": true, "OverrideAbstract": true, "ParcelCreator": true, "PluralsCandidate": true, "PropertyEscape": true, "ProtectedPermissions": true, "Range": true, "Registered": true, "RequiredSize": true, "ResAuto": true, "ResourceAsColor": true, "ResourceType": true, "ScrollViewCount": true, "ScrollViewSize": true, "ServiceExported": true, "SetTextI18n": true, "SimpleDateFormat": true, "SpUsage": true, "StopShip": true, "StringFormatCount": true, "StringFormatInvalid": true, "StringFormatMatches": true, "SupportAnnotationUsage": true, "SwitchIntDef": true, "TextFields": true, "TextViewEdits": true, "UnpackedNativeCode": true, "UnusedAttribute": true, "UnusedNamespace": true, "UseCompoundDrawables": true, "UsesMinSdkAttributes": true, "WrongCall": true, "WrongCase": true, "WrongFolder": true, "WrongManifestParent": true, "WrongRegion": true, "WrongThread": true, "WrongViewCast": true, "Assert": true, "SQLiteString": true, "LocalSuppress": true, "AddJavascriptInterface": true, "GetInstance": true, "EasterEgg": true, "ExportedContentProvider": true, "ExportedReceiver": true, "ExportedPreferenceActivity": true, "GrantAllUris": true, "HardcodedDebugMode": true, "InsecureBaseConfiguration": true, "SecureRandom": true, "TrustedServer": true, "UnprotectedSMSBroadcastReceiver": true, "UnsafeProtectedBroadcastReceiver": true, "UseCheckPermission": true, "WorldReadableFiles": true, "WorldWriteableFiles": true, "DisableBaselineAlignment": true, "DrawAllocation": true, "FieldGetter": true, "FloatMath": true, "HandlerLeak": true, "InefficientWeight": true, "MergeRootFrame": true, "NestedWeights": true, "Overdraw": true, "Recycle": true, "TooDeepLayout": true, "UselessLeaf": true, "ClickableViewAccessibility": true, "LabelFor": true, "ByteOrderMark": true, "RelativeOverlap": true, "IconColors": true, "IconDensities": true, "IconDipSize": true, "IconDuplicates": true, "IconDuplicatesConfig": true, "IconExpectedSize": true, "IconExtension": true, "IconLauncherShape": true, "IconLocation": true, "IconMissingDensityFolder": true, "IconMixedNinePatch": true, "IconNoDpi": true, "IconXmlAndPng": true, "ConvertToWebp": true, "GifUsage": true, "AppCompatResource": true, "AppIndexingError": true, "AppIndexingWarning": true, "BackButton": true, "ButtonCase": true, "ButtonOrder": true, "ButtonStyle": true, "GoogleAppIndexingDeepLinkError": true, "GoogleAppIndexingWarning": true, "NegativeMargin": true, "RtlCompat": true, "RtlEnabled": true, "RtlHardcoded": true, "RtlSymmetry": true, "RtlSuperscript": true, "AllowBackup": true, "AlwaysShowAction": true, "InvalidUsesTagAttribute": true, "OldTargetApi": true, "PermissionImpliesUnsupportedHardware": true, "UnsupportedChromeOsHardware": true, "MissingLeanbackLauncher": true, "MissingLeanbackSupport": true, "Typos": true, "TypographyDashes": true, "TypographyEllipsis": true, "TypographyFractions": true, "TypographyOther": true, "TypographyQuotes": true, "UnusedIds": true, "Autofill": true, "RestrictedApi": true, "VisibleForTests": true, "MissingInflatedId": true, "NotificationPermission": true, "ObsoleteSdkInt": true, "all": true}

// Confidence reports a tier-2 (medium) base confidence. This is an
// Android-lint port from AOSP; the detection relies on source-text
// patterns (call names, string literal contents, hardcoded allow-
// lists of API names) rather than type resolution, so project-
// specific wrapper APIs can cause false positives or negatives.
// Classified per roadmap/17.
func (r *LocalSuppressRule) Confidence() float64 { return 0.75 }

func (r *LocalSuppressRule) CheckLines(file *scanner.File) []scanner.Finding {
	var findings []scanner.Finding
	for i, line := range file.Lines {
		for _, m := range suppressLintRe.FindAllStringSubmatch(line, -1) {
			if !knownLintIssueIDs[m[1]] {
				findings = append(findings, r.Finding(file, i+1, 1, fmt.Sprintf("@SuppressLint(\"%s\"): '%s' is not a known Android Lint issue ID.", m[1], m[1])))
			}
		}
	}
	return findings
}

type PluralsCandidateRule struct {
	LineBase
	AndroidRule
}

var (
	pluralsIfRe   = regexp.MustCompile(`if\s*\(\s*\w+\s*==\s*1\s*\)`)
	pluralsWhenRe = regexp.MustCompile(`when\s*\(\s*(count|num|size|quantity|amount|number)\s*\)`)
)

// Confidence reports a tier-2 (medium) base confidence. This is an
// Android-lint port from AOSP; the detection relies on source-text
// patterns (call names, string literal contents, hardcoded allow-
// lists of API names) rather than type resolution, so project-
// specific wrapper APIs can cause false positives or negatives.
// Classified per roadmap/17.
func (r *PluralsCandidateRule) Confidence() float64 { return 0.75 }

func (r *PluralsCandidateRule) CheckLines(file *scanner.File) []scanner.Finding {
	var findings []scanner.Finding
	for i, line := range file.Lines {
		if scanner.IsCommentLine(line) {
			continue
		}
		if pluralsIfRe.MatchString(line) && containsStringFormatting(gatherWindow(file.Lines, i, 5)) {
			findings = append(findings, r.Finding(file, i+1, 1, "Manual pluralization detected. Use getQuantityString() for proper plural handling."))
		}
		if pluralsWhenRe.MatchString(line) && containsStringFormatting(gatherWindow(file.Lines, i, 10)) {
			findings = append(findings, r.Finding(file, i+1, 1, "Manual pluralization detected. Use getQuantityString() for proper plural handling."))
		}
	}
	return findings
}

func gatherWindow(lines []string, i, radius int) string {
	start := i - radius
	if start < 0 {
		start = 0
	}
	end := i + radius + 1
	if end > len(lines) {
		end = len(lines)
	}
	return strings.Join(lines[start:end], "\n")
}

func containsStringFormatting(text string) bool {
	for _, ind := range []string{"\"${", "getString(", "string.", "String.format", "resources.", "R.string.", "\"item", "\"file", "\"message", "\"photo", "\"comment", "\"result", "\"day", "\"hour", "\"minute", "\"second", "plural", "Plural"} {
		if strings.Contains(strings.ToLower(text), strings.ToLower(ind)) {
			return true
		}
	}
	return false
}
