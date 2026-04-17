package rules

// Android Lint rules for Security, Performance, Accessibility, I18N, and RTL categories.
// Ported from AOSP Android Lint.
// Origin: https://android.googlesource.com/platform/tools/base/+/refs/heads/main/lint/libs/lint-checks/

import (
	"regexp"
	"strings"

	"github.com/kaeawc/krit/internal/scanner"
)

// Additional category constants not in android.go
const (
	ALCRTL AndroidLintCategory = "rtl"
)

// =============================================================================
// Security Rules
// =============================================================================

// AddJavascriptInterfaceRule detects addJavascriptInterface() calls.
type AddJavascriptInterfaceRule struct{ AndroidRule }

// Confidence reports a tier-2 (medium) base confidence. This is an
// Android-lint port from AOSP; the detection relies on source-text
// patterns (call names, string literal contents, hardcoded allow-
// lists of API names) rather than type resolution, so project-
// specific wrapper APIs can cause false positives or negatives.
// Classified per roadmap/17.
func (r *AddJavascriptInterfaceRule) Confidence() float64 { return 0.75 }

func (r *AddJavascriptInterfaceRule) CheckLines(file *scanner.File) []scanner.Finding {
	var findings []scanner.Finding
	for i, line := range file.Lines {
		if strings.Contains(line, "addJavascriptInterface(") || strings.Contains(line, "addJavascriptInterface (") {
			findings = append(findings, r.Finding(file, i+1, 1,
				"addJavascriptInterface called. This can introduce XSS vulnerabilities on older Android versions."))
		}
	}
	return findings
}


// GetInstanceRule detects Cipher.getInstance with insecure algorithms (ECB, DES).
type GetInstanceRule struct{ AndroidRule }

var getInstanceRe = regexp.MustCompile(`Cipher\.getInstance\s*\(\s*"([^"]*)"`)

// Confidence reports a tier-2 (medium) base confidence. This is an
// Android-lint port from AOSP; the detection relies on source-text
// patterns (call names, string literal contents, hardcoded allow-
// lists of API names) rather than type resolution, so project-
// specific wrapper APIs can cause false positives or negatives.
// Classified per roadmap/17.
func (r *GetInstanceRule) Confidence() float64 { return 0.75 }

func (r *GetInstanceRule) CheckLines(file *scanner.File) []scanner.Finding {
	var findings []scanner.Finding
	for i, line := range file.Lines {
		matches := getInstanceRe.FindStringSubmatch(line)
		if matches != nil {
			algo := strings.ToUpper(matches[1])
			if strings.Contains(algo, "ECB") || strings.HasPrefix(algo, "DES") {
				findings = append(findings, r.Finding(file, i+1, 1,
					"Cipher.getInstance uses insecure algorithm. Avoid ECB mode and DES."))
			}
		}
	}
	return findings
}


// EasterEggRule detects comments containing easter egg references.
type EasterEggRule struct{ AndroidRule }

var easterEggRe = regexp.MustCompile(`(?i)\b(easter\s*egg|cheat\s*code|secret\s*(?:mode|menu|feature))\b`)

// Confidence reports a tier-2 (medium) base confidence. This is an
// Android-lint port from AOSP; the detection relies on source-text
// patterns (call names, string literal contents, hardcoded allow-
// lists of API names) rather than type resolution, so project-
// specific wrapper APIs can cause false positives or negatives.
// Classified per roadmap/17.
func (r *EasterEggRule) Confidence() float64 { return 0.75 }

func (r *EasterEggRule) CheckLines(file *scanner.File) []scanner.Finding {
	var findings []scanner.Finding
	for i, line := range file.Lines {
		// Only check comments
		if scanner.IsCommentLine(line) {
			if easterEggRe.MatchString(line) {
				findings = append(findings, r.Finding(file, i+1, 1,
					"Code contains easter egg / hidden feature reference. Review for security implications."))
			}
		}
	}
	return findings
}


// ExportedContentProviderRule detects exported content providers without permission.
type ExportedContentProviderRule struct{ AndroidRule }

var contentProviderRe = regexp.MustCompile(`:\s*ContentProvider\s*\(`)

var permissionEnforcementRe = regexp.MustCompile(`(?i)enforceCallingPermission|enforceCallingOrSelfPermission|checkCallingPermission|checkCallingOrSelfPermission|enforcePermission|checkPermission`)

// Confidence reports a tier-2 (medium) base confidence. This is an
// Android-lint port from AOSP; the detection relies on source-text
// patterns (call names, string literal contents, hardcoded allow-
// lists of API names) rather than type resolution, so project-
// specific wrapper APIs can cause false positives or negatives.
// Classified per roadmap/17.
func (r *ExportedContentProviderRule) Confidence() float64 { return 0.75 }

func (r *ExportedContentProviderRule) CheckLines(file *scanner.File) []scanner.Finding {
	// Skip if the file has any permission enforcement calls
	for _, line := range file.Lines {
		if permissionEnforcementRe.MatchString(line) {
			return nil
		}
	}
	var findings []scanner.Finding
	for i, line := range file.Lines {
		if contentProviderRe.MatchString(line) {
			findings = append(findings, r.Finding(file, i+1, 1,
				"ContentProvider subclass may be exported without permission. Ensure permissions are enforced."))
		}
	}
	return findings
}


// ExportedReceiverRule detects exported receivers without permission.
type ExportedReceiverRule struct{ AndroidRule }

var broadcastReceiverRe = regexp.MustCompile(`:\s*BroadcastReceiver\s*\(`)

// Confidence reports a tier-2 (medium) base confidence. This is an
// Android-lint port from AOSP; the detection relies on source-text
// patterns (call names, string literal contents, hardcoded allow-
// lists of API names) rather than type resolution, so project-
// specific wrapper APIs can cause false positives or negatives.
// Classified per roadmap/17.
func (r *ExportedReceiverRule) Confidence() float64 { return 0.75 }

func (r *ExportedReceiverRule) CheckLines(file *scanner.File) []scanner.Finding {
	var findings []scanner.Finding
	for i, line := range file.Lines {
		if broadcastReceiverRe.MatchString(line) {
			findings = append(findings, r.Finding(file, i+1, 1,
				"BroadcastReceiver subclass may be exported without permission. Ensure permissions are enforced."))
		}
	}
	return findings
}


// GrantAllUrisRule detects overly broad URI permissions.
type GrantAllUrisRule struct{ AndroidRule }

var grantUriRe = regexp.MustCompile(`\bgrantUriPermission[s]?\b`)

// Confidence reports a tier-2 (medium) base confidence. This is an
// Android-lint port from AOSP; the detection relies on source-text
// patterns (call names, string literal contents, hardcoded allow-
// lists of API names) rather than type resolution, so project-
// specific wrapper APIs can cause false positives or negatives.
// Classified per roadmap/17.
func (r *GrantAllUrisRule) Confidence() float64 { return 0.75 }

func (r *GrantAllUrisRule) CheckLines(file *scanner.File) []scanner.Finding {
	var findings []scanner.Finding
	for i, line := range file.Lines {
		if grantUriRe.MatchString(line) {
			findings = append(findings, r.Finding(file, i+1, 1,
				"Overly broad URI permission grant. Consider restricting to specific URIs."))
		}
	}
	return findings
}


// SecureRandomRule detects java.util.Random usage where SecureRandom should be used.
type SecureRandomRule struct{ AndroidRule }

var secureRandomImportRe = regexp.MustCompile(`import\s+java\.util\.Random\b`)
var randomInstantiationRe = regexp.MustCompile(`\bjava\.util\.Random\s*\(|(?:^|[^.])\bRandom\s*\(\s*\)`)

// Confidence reports a tier-2 (medium) base confidence. This is an
// Android-lint port from AOSP; the detection relies on source-text
// patterns (call names, string literal contents, hardcoded allow-
// lists of API names) rather than type resolution, so project-
// specific wrapper APIs can cause false positives or negatives.
// Classified per roadmap/17.
func (r *SecureRandomRule) Confidence() float64 { return 0.75 }

func (r *SecureRandomRule) CheckLines(file *scanner.File) []scanner.Finding {
	var findings []scanner.Finding
	hasInsecureImport := false
	for _, line := range file.Lines {
		if secureRandomImportRe.MatchString(line) {
			hasInsecureImport = true
			break
		}
	}
	for i, line := range file.Lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "//") || strings.HasPrefix(trimmed, "import ") {
			continue
		}
		// Detect direct java.util.Random usage
		if strings.Contains(line, "java.util.Random(") {
			findings = append(findings, r.Finding(file, i+1, 1,
				"Using java.util.Random. Use java.security.SecureRandom for security-sensitive operations."))
			continue
		}
		// Detect Random() instantiation when java.util.Random is imported
		if hasInsecureImport && strings.Contains(line, "Random(") &&
			!strings.Contains(line, "SecureRandom") &&
			!strings.Contains(line, "ThreadLocalRandom") {
			findings = append(findings, r.Finding(file, i+1, 1,
				"Using java.util.Random. Use java.security.SecureRandom for security-sensitive operations."))
		}
	}
	return findings
}


// TrustedServerRule detects trust-all certificate patterns.
type TrustedServerRule struct{ AndroidRule }

// Confidence reports a tier-2 (medium) base confidence. This is an
// Android-lint port from AOSP; the detection relies on source-text
// patterns (call names, string literal contents, hardcoded allow-
// lists of API names) rather than type resolution, so project-
// specific wrapper APIs can cause false positives or negatives.
// Classified per roadmap/17.
func (r *TrustedServerRule) Confidence() float64 { return 0.75 }

func (r *TrustedServerRule) CheckLines(file *scanner.File) []scanner.Finding {
	var findings []scanner.Finding
	for i, line := range file.Lines {
		// Detect common trust-all patterns
		if strings.Contains(line, "TrustAllCertificates") ||
			strings.Contains(line, "AllowAllHostnameVerifier") ||
			strings.Contains(line, "ALLOW_ALL_HOSTNAME_VERIFIER") ||
			strings.Contains(line, "trustAllCerts") ||
			strings.Contains(line, "X509TrustManager") && strings.Contains(line, "object") {
			findings = append(findings, r.Finding(file, i+1, 1,
				"Trusting all certificates or hostnames is insecure. Use proper certificate validation."))
		}
	}
	return findings
}


// WorldReadableFilesRule detects MODE_WORLD_READABLE usage.
type WorldReadableFilesRule struct{ AndroidRule }

// Confidence reports a tier-2 (medium) base confidence. This is an
// Android-lint port from AOSP; the detection relies on source-text
// patterns (call names, string literal contents, hardcoded allow-
// lists of API names) rather than type resolution, so project-
// specific wrapper APIs can cause false positives or negatives.
// Classified per roadmap/17.
func (r *WorldReadableFilesRule) Confidence() float64 { return 0.75 }

func (r *WorldReadableFilesRule) CheckLines(file *scanner.File) []scanner.Finding {
	var findings []scanner.Finding
	for i, line := range file.Lines {
		if strings.Contains(line, "MODE_WORLD_READABLE") {
			findings = append(findings, r.Finding(file, i+1, 1,
				"MODE_WORLD_READABLE is insecure. Use more restrictive file permissions."))
		}
	}
	return findings
}


// WorldWriteableFilesRule detects MODE_WORLD_WRITEABLE usage.
type WorldWriteableFilesRule struct{ AndroidRule }

// Confidence reports a tier-2 (medium) base confidence. This is an
// Android-lint port from AOSP; the detection relies on source-text
// patterns (call names, string literal contents, hardcoded allow-
// lists of API names) rather than type resolution, so project-
// specific wrapper APIs can cause false positives or negatives.
// Classified per roadmap/17.
func (r *WorldWriteableFilesRule) Confidence() float64 { return 0.75 }

func (r *WorldWriteableFilesRule) CheckLines(file *scanner.File) []scanner.Finding {
	var findings []scanner.Finding
	for i, line := range file.Lines {
		if strings.Contains(line, "MODE_WORLD_WRITEABLE") || strings.Contains(line, "MODE_WORLD_WRITABLE") {
			findings = append(findings, r.Finding(file, i+1, 1,
				"MODE_WORLD_WRITEABLE is insecure. Use more restrictive file permissions."))
		}
	}
	return findings
}


// =============================================================================
// Performance Rules
// =============================================================================

// DrawAllocationRule detects object allocations inside onDraw/draw methods.
type DrawAllocationRule struct{ AndroidRule }

var drawAllocRe = regexp.MustCompile(`\b(?:Paint|Rect|RectF|Path|Matrix|LinearGradient|RadialGradient|Bitmap|Canvas|PorterDuffXfermode|Shader|ColorFilter)\s*\(`)

// Confidence reports a tier-2 (medium) base confidence. This is an
// Android-lint port from AOSP; the detection relies on source-text
// patterns (call names, string literal contents, hardcoded allow-
// lists of API names) rather than type resolution, so project-
// specific wrapper APIs can cause false positives or negatives.
// Classified per roadmap/17.
func (r *DrawAllocationRule) Confidence() float64 { return 0.75 }

func (r *DrawAllocationRule) CheckLines(file *scanner.File) []scanner.Finding {
	var findings []scanner.Finding
	inDraw := false
	braceDepth := 0
	drawStartDepth := 0

	for i, line := range file.Lines {
		trimmed := strings.TrimSpace(line)

		// Detect onDraw or draw override
		if !inDraw && (strings.Contains(trimmed, "override fun onDraw(") ||
			strings.Contains(trimmed, "override fun draw(") ||
			strings.Contains(trimmed, "fun onDraw(") ||
			strings.Contains(trimmed, "fun draw(canvas")) {
			inDraw = true
			drawStartDepth = braceDepth
		}

		braceDepth += strings.Count(line, "{") - strings.Count(line, "}")

		if inDraw {
			if drawAllocRe.MatchString(trimmed) && !strings.HasPrefix(trimmed, "//") {
				findings = append(findings, r.Finding(file, i+1, 1,
					"Allocation in drawing code. Move allocations out of onDraw() for better performance."))
			}
			if braceDepth <= drawStartDepth {
				inDraw = false
			}
		}
	}
	return findings
}


// FieldGetterRule detects using getter instead of direct field access in loops.
type FieldGetterRule struct{ AndroidRule }

var fieldGetterCallRe = regexp.MustCompile(`\.get[A-Z]\w*\(`)

// Confidence reports a tier-2 (medium) base confidence. This is an
// Android-lint port from AOSP; the detection relies on source-text
// patterns (call names, string literal contents, hardcoded allow-
// lists of API names) rather than type resolution, so project-
// specific wrapper APIs can cause false positives or negatives.
// Classified per roadmap/17.
func (r *FieldGetterRule) Confidence() float64 { return 0.75 }

func (r *FieldGetterRule) CheckLines(file *scanner.File) []scanner.Finding {
	var findings []scanner.Finding
	inLoop := false
	braceDepth := 0
	loopStartDepth := 0
	for i, line := range file.Lines {
		trimmed := strings.TrimSpace(line)
		if !inLoop {
			if strings.HasPrefix(trimmed, "for ") || strings.HasPrefix(trimmed, "for(") ||
				strings.HasPrefix(trimmed, "while ") || strings.HasPrefix(trimmed, "while(") {
				inLoop = true
				loopStartDepth = braceDepth
			}
		}
		braceDepth += strings.Count(line, "{") - strings.Count(line, "}")
		if inLoop {
			if fieldGetterCallRe.MatchString(line) && !strings.HasPrefix(trimmed, "//") {
				findings = append(findings, r.Finding(file, i+1, 1,
					"Getter call inside loop. Use direct field access for better performance."))
			}
			if braceDepth <= loopStartDepth {
				inLoop = false
			}
		}
	}
	return findings
}


// FloatMathRule detects deprecated FloatMath usage.
type FloatMathRule struct{ AndroidRule }

// Confidence reports a tier-2 (medium) base confidence. This is an
// Android-lint port from AOSP; the detection relies on source-text
// patterns (call names, string literal contents, hardcoded allow-
// lists of API names) rather than type resolution, so project-
// specific wrapper APIs can cause false positives or negatives.
// Classified per roadmap/17.
func (r *FloatMathRule) Confidence() float64 { return 0.75 }

func (r *FloatMathRule) CheckLines(file *scanner.File) []scanner.Finding {
	var findings []scanner.Finding
	for i, line := range file.Lines {
		if strings.Contains(line, "FloatMath.") {
			findings = append(findings, r.Finding(file, i+1, 1,
				"FloatMath is deprecated. Use kotlin.math or java.lang.Math instead."))
		}
	}
	return findings
}


// HandlerLeakRule detects non-static inner Handler classes that may leak.
type HandlerLeakRule struct{ AndroidRule }

var handlerClassRe = regexp.MustCompile(`(?:inner\s+)?class\s+\w+.*:\s*Handler\s*\(`)
var handlerInnerRe = regexp.MustCompile(`\binner\s+class\s+\w+`)

// Confidence reports a tier-2 (medium) base confidence. This is an
// Android-lint port from AOSP; the detection relies on source-text
// patterns (call names, string literal contents, hardcoded allow-
// lists of API names) rather than type resolution, so project-
// specific wrapper APIs can cause false positives or negatives.
// Classified per roadmap/17.
func (r *HandlerLeakRule) Confidence() float64 { return 0.75 }

func (r *HandlerLeakRule) CheckLines(file *scanner.File) []scanner.Finding {
	var findings []scanner.Finding
	for i, line := range file.Lines {
		if handlerClassRe.MatchString(line) && handlerInnerRe.MatchString(line) {
			findings = append(findings, r.Finding(file, i+1, 1,
				"This Handler class should be static or leaks might occur. Use a WeakReference to the outer class."))
		}
		// Also detect anonymous Handler() object expressions
		if strings.Contains(line, "object : Handler(") {
			findings = append(findings, r.Finding(file, i+1, 1,
				"Anonymous Handler may leak the enclosing class. Use a static inner class with a WeakReference."))
		}
	}
	return findings
}


// RecycleRule detects missing recycle()/close() calls for resources.
type RecycleRule struct{ AndroidRule }

var recycleTargets = []string{
	"obtainStyledAttributes", "obtainAttributes",
	"obtainTypedArray", "obtain(",
}
var recycleTypes = []string{
	"TypedArray", "Cursor", "VelocityTracker", "Parcel",
}

// Confidence reports a tier-2 (medium) base confidence. This is an
// Android-lint port from AOSP; the detection relies on source-text
// patterns (call names, string literal contents, hardcoded allow-
// lists of API names) rather than type resolution, so project-
// specific wrapper APIs can cause false positives or negatives.
// Classified per roadmap/17.
func (r *RecycleRule) Confidence() float64 { return 0.75 }

func (r *RecycleRule) CheckLines(file *scanner.File) []scanner.Finding {
	var findings []scanner.Finding
	fullContent := strings.Join(file.Lines, "\n")

	for i, line := range file.Lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "//") {
			continue
		}
		for _, target := range recycleTargets {
			if strings.Contains(line, target) {
				// Check if recycle() or close() appears in the same function scope
				if !strings.Contains(fullContent, "recycle()") && !strings.Contains(fullContent, ".close()") {
					findings = append(findings, r.Finding(file, i+1, 1,
						"Resource obtained but recycle()/close() not found. Ensure the resource is properly released."))
				}
			}
		}
		for _, typ := range recycleTypes {
			// Detect variable declarations like: val x = TypedArray or val x: TypedArray
			if strings.Contains(line, typ) && (strings.Contains(line, "val ") || strings.Contains(line, "var ")) {
				if !strings.Contains(fullContent, ".recycle()") && !strings.Contains(fullContent, ".close()") &&
					!strings.Contains(fullContent, ".use {") && !strings.Contains(fullContent, ".use{") {
					findings = append(findings, r.Finding(file, i+1, 1,
						typ+" acquired but no recycle()/close()/use{} found. Ensure the resource is properly released."))
				}
			}
		}
	}
	return findings
}


// =============================================================================
// I18N Rules
// =============================================================================

// ByteOrderMarkRule detects BOM (byte order mark) in files.
type ByteOrderMarkRule struct{ AndroidRule }

// Confidence bumps this line rule from the 0.75 line-rule default to
// 0.95 — the BOM check is a literal three-byte compare at the start
// of the file content. No heuristic path.
func (r *ByteOrderMarkRule) Confidence() float64 { return 0.95 }

func (r *ByteOrderMarkRule) CheckLines(file *scanner.File) []scanner.Finding {
	// BOM is the first 3 bytes: EF BB BF (UTF-8 BOM)
	if len(file.Content) >= 3 &&
		file.Content[0] == 0xEF && file.Content[1] == 0xBB && file.Content[2] == 0xBF {
		return []scanner.Finding{r.Finding(file, 1, 1,
			"File contains a UTF-8 byte order mark (BOM). Remove the BOM for consistency.")}
	}
	return nil
}

