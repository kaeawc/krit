package rules

import (
	"fmt"
	"regexp"
	"strings"

	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// KotlinOptionsDslRule flags the removed `kotlinOptions` Gradle DSL.
//
// Sources (kotlinlang.org):
//   - compatibility-guide-22.html#deprecate-kotlinoptions-dsl: warning in 2.0.0,
//     error for the DSL in 2.2.0.
//   - compatibility-guide-23.html: error for every `kotlinOptions` property in 2.3.0.
//   - gradle-compiler-options.html#migrate-from-kotlinoptions-to-compileroptions:
//     the migration, including `JvmTarget`/`KotlinVersion` typed values and
//     `+=` becoming `add()`/`addAll()`.
//   - developer.android.com/build/migrate-to-built-in-kotlin: `android.kotlinOptions{}`
//     becomes the top-level `kotlin { compilerOptions {} }`.
//
// The finding severity follows the project's Kotlin version: error from 2.2.0,
// warning for 2.0 and 2.1, and a medium-confidence warning when the version is unknown.
// Versions below 2.0 are skipped because `kotlinOptions` was not yet deprecated.
type KotlinOptionsDslRule struct {
	GradleBase
	BaseRule
}

func (r *KotlinOptionsDslRule) Confidence() float64 { return api.ConfidenceHigh }

var (
	kotlinOptionsRe        = regexp.MustCompile(`\bkotlinOptions\b`)
	kotlinOptionsDeclRe    = regexp.MustCompile(`(?:\bval|\bvar|\bfun|\bdef)\s+$`)
	kotlinOptionsTaskHead  = regexp.MustCompile(`KotlinCompile|KotlinJvmCompile|\bcompile\w*Kotlin\w*\b|withType\s*<?\s*\(?\s*\w*Kotlin\w*`)
	kotlinOptionsAndroidHd = regexp.MustCompile(`(?:^|[.\s])android$`)

	koJvmTargetRe = regexp.MustCompile(`^jvmTarget\s*=\s*"(\d+(?:\.\d+)?)"$`)
	koFreeArgsRe  = regexp.MustCompile(`^freeCompilerArgs\s*\+=\s*(listOf\(.*\)|"[^"]*")$`)
	koBooleanRe   = regexp.MustCompile(`^(allWarningsAsErrors|suppressWarnings|verbose|javaParameters)\s*=\s*(true|false)$`)
	koKotlinVerRe = regexp.MustCompile(`^(languageVersion|apiVersion)\s*=\s*"(\d+\.\d+)"$`)
)

const (
	koJvmTargetType = "org.jetbrains.kotlin.gradle.dsl.JvmTarget"
	koVersionType   = "org.jetbrains.kotlin.gradle.dsl.KotlinVersion"
)

func (r *KotlinOptionsDslRule) check(ctx *api.Context) {
	path := ctx.GradlePath
	if isGradleSettingsScript(path) {
		return
	}
	src := newGradleSource(ctx.GradleContent)
	locs := kotlinOptionsRe.FindAllStringIndex(src.code, -1)
	if len(locs) == 0 {
		return
	}
	tool := gradleToolchainFor(ctx)
	if tool.kotlin.known && !tool.kotlin.atLeast(2, 0, 0) {
		return
	}
	for _, loc := range locs {
		f, fix, ok := r.finding(ctx, src, tool, loc[0], loc[1])
		if !ok {
			continue
		}
		f.Fix = fix
		ctx.Emit(f)
	}
}

func (r *KotlinOptionsDslRule) finding(ctx *api.Context, src *gradleSource, tool gradleToolchain, start, end int) (scanner.Finding, *scanner.Fix, bool) {
	if kotlinOptionsDeclRe.MatchString(src.code[:start]) {
		return scanner.Finding{}, nil, false
	}
	rest := strings.TrimLeft(src.code[end:], " \t")
	if strings.HasPrefix(rest, "=") && !strings.HasPrefix(rest, "==") {
		return scanner.Finding{}, nil, false
	}
	header := src.enclosingHeader(start)
	inAndroid := kotlinOptionsAndroidHd.MatchString(header)
	inTask := !inAndroid && kotlinOptionsTaskHead.MatchString(header)

	var fix *scanner.Fix
	if inTask && isKotlinDslScript(ctx.GradlePath) {
		fix = kotlinOptionsFix(src, start, end)
	}

	line, col := src.lineCol(start)
	sev, conf, intro := kotlinOptionsSeverity(tool)
	msg := "`kotlinOptions` " + intro + " Use `compilerOptions {}` instead."
	if inAndroid {
		msg += " Configure it in the top-level `kotlin { compilerOptions { ... } }` block."
	}
	return scanner.Finding{
		File: ctx.GradlePath, Line: line, Col: col, StartByte: start, EndByte: end,
		Severity: sev, Confidence: conf, Message: msg,
	}, fix, true
}

// kotlinOptionsSeverity maps the project's Kotlin version to the finding
// severity, confidence and the sentence describing the construct's status.
func kotlinOptionsSeverity(tool gradleToolchain) (severity string, confidence float64, status string) {
	switch {
	case tool.kotlin.atLeast(2, 3, 0):
		return "error", api.ConfidenceHigh, "is an error since Kotlin 2.2.0 (every property too since 2.3.0)."
	case tool.kotlin.atLeast(2, 2, 0):
		return "error", api.ConfidenceHigh, "is an error since Kotlin 2.2.0."
	case tool.kotlin.known:
		return "warning", api.ConfidenceHigh, "is deprecated since Kotlin 2.0.0 and an error since 2.2.0."
	default:
		return "warning", api.ConfidenceMedium, "is deprecated since Kotlin 2.0.0 and an error since 2.2.0; the project's Kotlin version is not known."
	}
}

// kotlinOptionsFix rewrites a `kotlinOptions {}` block or a `kotlinOptions.x = v`
// statement when every statement has a direct `compilerOptions` equivalent.
func kotlinOptionsFix(src *gradleSource, start, end int) *scanner.Fix {
	rest := src.code[end:]
	trimmed := strings.TrimLeft(rest, " \t")
	switch {
	case strings.HasPrefix(trimmed, "{"):
		open := end + (len(rest) - len(trimmed))
		closeIdx := src.matchingBrace(open)
		if closeIdx < 0 || src.content[open:closeIdx] != src.text[open:closeIdx] {
			return nil
		}
		body, ok := mapKotlinOptionsBody(src.content[open+1 : closeIdx])
		if !ok {
			return nil
		}
		repl := "compilerOptions" + src.content[end:open+1] + body + "}"
		return &scanner.Fix{ByteMode: true, StartByte: start, EndByte: closeIdx + 1, Replacement: repl}
	case strings.HasPrefix(trimmed, "."):
		dot := end + (len(rest) - len(trimmed))
		_, lineEnd := src.lineBounds(start)
		stmtEnd := start + len(strings.TrimRight(src.code[start:lineEnd], " \t"))
		if stmtEnd <= dot+1 || src.content[start:stmtEnd] != src.text[start:stmtEnd] {
			return nil
		}
		mapped, ok := mapKotlinOptionsStatement(strings.TrimSpace(src.content[dot+1 : stmtEnd]))
		if !ok {
			return nil
		}
		return &scanner.Fix{ByteMode: true, StartByte: start, EndByte: stmtEnd, Replacement: "compilerOptions." + mapped}
	}
	return nil
}

// mapKotlinOptionsBody maps each line of a block body, keeping indentation. It
// fails when a non-blank line has no mapping.
func mapKotlinOptionsBody(body string) (string, bool) {
	lines := strings.Split(body, "\n")
	for i, line := range lines {
		stmt := strings.TrimSpace(line)
		if stmt == "" {
			continue
		}
		mapped, ok := mapKotlinOptionsStatement(stmt)
		if !ok {
			return "", false
		}
		lead := line[:strings.Index(line, stmt)]
		trail := line[strings.Index(line, stmt)+len(stmt):]
		lines[i] = lead + mapped + trail
	}
	return strings.Join(lines, "\n"), true
}

// mapKotlinOptionsStatement converts one `kotlinOptions` property assignment to
// its `compilerOptions` property form.
func mapKotlinOptionsStatement(stmt string) (string, bool) {
	switch {
	case koJvmTargetRe.MatchString(stmt):
		m := koJvmTargetRe.FindStringSubmatch(stmt)
		return fmt.Sprintf("jvmTarget.set(%s.fromTarget(%q))", koJvmTargetType, m[1]), true
	case koFreeArgsRe.MatchString(stmt):
		m := koFreeArgsRe.FindStringSubmatch(stmt)
		if strings.HasPrefix(m[1], "listOf(") {
			return "freeCompilerArgs.addAll(" + m[1] + ")", true
		}
		return "freeCompilerArgs.add(" + m[1] + ")", true
	case koBooleanRe.MatchString(stmt):
		m := koBooleanRe.FindStringSubmatch(stmt)
		return fmt.Sprintf("%s.set(%s)", m[1], m[2]), true
	case koKotlinVerRe.MatchString(stmt):
		m := koKotlinVerRe.FindStringSubmatch(stmt)
		return fmt.Sprintf("%s.set(%s.fromVersion(%q))", m[1], koVersionType, m[2]), true
	}
	return "", false
}
