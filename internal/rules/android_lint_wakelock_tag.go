package rules

import (
	"fmt"
	"strings"

	"github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

const powerManagerClass = "android.os.PowerManager"

// InvalidWakeLockTagRule mirrors Lint's InvalidWakeLockTag: a constant tag passed to PowerManager.newWakeLock that is
// empty, reserved (`*name*`), or lacks a `prefix:` colon.
type InvalidWakeLockTagRule struct {
	FlatDispatchBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence: the receiver type is inferred from declarations.
func (r *InvalidWakeLockTagRule) Confidence() float64 { return api.ConfidenceMedium }

func (r *InvalidWakeLockTagRule) check(ctx *api.Context) {
	file := ctx.File
	call, ok := lintCallAt(file, ctx.Idx)
	if !ok || call.Name != "newWakeLock" || len(call.Args) < 2 || call.ReceiverNode == 0 || !isPowerManagerReceiver(ctx, call) {
		return
	}
	tagNode := call.Arg(1, "tag")
	tag, known := lintEvalStringStrict(file, tagNode)
	if !known {
		return
	}
	if msg := wakeLockTagError(tag); msg != "" {
		ctx.Emit(lintFindingAtNode(ctx, ctx.Idx, "", msg))
	}
}

// wakeLockTagError returns Lint's message for an invalid tag, or "" when the tag is acceptable.
func wakeLockTagError(tag string) string {
	switch {
	case tag == "":
		return "Tag name should not be empty to make wake lock problems easier to debug"
	case len(tag) >= 2 && tag[0] == '*' && strings.IndexByte(tag[1:], '*') >= 0:
		return fmt.Sprintf("`%s` is a reserved platform tag name and cannot be used", tag)
	case !strings.Contains(tag, ":"):
		return fmt.Sprintf("Tag name should use a unique prefix followed by a colon (found %s). For instance `myapp:mywakelocktag`. This will help with debugging", tag)
	}
	return ""
}

// isPowerManagerReceiver reports whether the call's receiver is a PowerManager, by resolved type, declared type, or
// a `getSystemService` initializer that names it.
func isPowerManagerReceiver(ctx *api.Context, call lintCall) bool {
	if lintReceiverIsType(ctx, call, powerManagerClass) {
		return true
	}
	file := ctx.File
	recv := call.ReceiverNode
	switch file.FlatType(recv) {
	case "simple_identifier", "identifier":
		init := lintInitializerOf(file, call.Idx, file.FlatNodeText(recv), true)
		return init != 0 && initNamesPowerManager(file.FlatNodeText(init))
	case "parenthesized_expression", "as_expression", "cast_expression", "call_expression", "method_invocation":
		return initNamesPowerManager(file.FlatNodeText(recv))
	}
	return false
}

// initNamesPowerManager matches `x as PowerManager`, `(PowerManager) x`, `getSystemService<PowerManager>()` and
// `getSystemService(PowerManager::class.java)`.
func initNamesPowerManager(text string) bool {
	if !strings.Contains(text, "getSystemService") && !strings.Contains(text, " as ") && !strings.Contains(text, "(PowerManager)") {
		return false
	}
	return strings.Contains(text, "PowerManager")
}

// lintEvalStringStrict folds a string made only of constants: literals without interpolation, `+` concatenation, and
// declarations in this file that nothing reassigns. It reports false when any part is unknown.
func lintEvalStringStrict(file *scanner.File, idx uint32) (string, bool) {
	return lintEvalStringStrictDepth(file, idx, idx, 0)
}

func lintEvalStringStrictDepth(file *scanner.File, idx, scope uint32, depth int) (string, bool) {
	if idx == 0 || depth > 6 {
		return "", false
	}
	switch file.FlatType(idx) {
	case "string_literal":
		if flatContainsStringInterpolation(file, idx) {
			return "", false
		}
		return lintStringLiteralValue(file, idx), true
	case "parenthesized_expression":
		for child := file.FlatFirstChild(idx); child != 0; child = file.FlatNextSib(child) {
			if file.FlatIsNamed(child) {
				return lintEvalStringStrictDepth(file, child, scope, depth)
			}
		}
	case "additive_expression", "binary_expression":
		var sb strings.Builder
		for child := file.FlatFirstChild(idx); child != 0; child = file.FlatNextSib(child) {
			if !file.FlatIsNamed(child) {
				if file.FlatNodeText(child) != "+" {
					return "", false
				}
				continue
			}
			part, ok := lintEvalStringStrictDepth(file, child, scope, depth)
			if !ok {
				return "", false
			}
			sb.WriteString(part)
		}
		return sb.String(), true
	case "simple_identifier", "identifier":
		if init := lintInitializerOf(file, scope, file.FlatNodeText(idx), true); init != 0 && !lintNameAssignedInFile(file, file.FlatNodeText(idx)) {
			return lintEvalStringStrictDepth(file, init, init, depth+1)
		}
	}
	return "", false
}

// lintNameAssignedInFile reports whether any assignment or increment in the file targets name.
func lintNameAssignedInFile(file *scanner.File, name string) bool {
	return lintNameReassigned(file, 0, name)
}
