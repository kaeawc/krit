package rules

import (
	"testing"

	"github.com/kaeawc/krit/internal/scanner"
	"github.com/kaeawc/krit/internal/typeinfer"
)

// resolveFirstElvisOperandType parses code with a source resolver, finds the
// first elvis_expression the rule would consider, and returns the result of
// uselessElvisResolvedOperandType for its left operand. `present` reports
// whether a candidate elvis operand was found at all.
func resolveFirstElvisOperandType(t *testing.T, code string) (rt *typeinfer.ResolvedType, ok, present bool) {
	t.Helper()
	file := parseInlineForInternalTest(t, code)
	resolver := typeinfer.NewResolver()
	resolver.IndexFilesParallel([]*scanner.File{file}, 1)

	var operand uint32
	file.FlatWalkNodes(0, "elvis_expression", func(idx uint32) {
		if present {
			return
		}
		if _, op, accepted := uselessElvisOperand(file, idx); accepted {
			operand = op
			present = true
		}
	})
	if !present {
		return nil, false, false
	}
	rt, ok = uselessElvisResolvedOperandType(file, resolver, operand)
	return rt, ok, true
}

// TestUselessElvisResolvedOperandType_UnknownReceiverNavigation pins the
// regression: a member-access elvis operand whose receiver type is not
// declared in this file (`decl.source`) must NOT be treated as a proven
// non-null type. Source inference defaults such an unresolved member access
// to non-null, which previously made the rule flag the elvis fallback as dead
// code even though the member may well be nullable.
func TestUselessElvisResolvedOperandType_UnknownReceiverNavigation(t *testing.T) {
	code := "fun f(decl: ExternalDecl): Any { val y = decl.source ?: return Unit; return y }\n"
	_, ok, present := resolveFirstElvisOperandType(t, code)
	if !present {
		t.Fatal("expected to find an elvis operand for `decl.source ?: ...`")
	}
	if ok {
		t.Fatal("member access on a receiver whose type is not declared in-file must not resolve to a proven type")
	}
}

// TestUselessElvisResolvedOperandType_UnknownMemberOnKnownClass covers a
// navigation to a member that does not exist on an in-file class. The
// receiver type resolves, but the member does not, so its nullability is
// unproven and the operand must not be accepted.
func TestUselessElvisResolvedOperandType_UnknownMemberOnKnownClass(t *testing.T) {
	code := "class Holder { val label: String = \"\" }\n" +
		"fun f(): Any { val h = Holder(); val y = h.missing ?: \"fallback\"; return y }\n"
	_, ok, present := resolveFirstElvisOperandType(t, code)
	if !present {
		t.Fatal("expected to find an elvis operand for `h.missing ?: ...`")
	}
	if ok {
		t.Fatal("navigation to a member absent from a known in-file class must not resolve to a proven type")
	}
}

// TestUselessElvisResolvedOperandType_ProvenInFileMember is the positive
// guard: a member access whose receiver is a local val of an in-file class
// and whose member is a non-null property must still resolve to a proven
// non-null type so the rule keeps firing on genuinely dead fallbacks.
func TestUselessElvisResolvedOperandType_ProvenInFileMember(t *testing.T) {
	code := "class Holder { val label: String = \"\" }\n" +
		"fun f(): Any { val h = Holder(); val y = h.label ?: \"fallback\"; return y }\n"
	rt, ok, present := resolveFirstElvisOperandType(t, code)
	if !present {
		t.Fatal("expected to find an elvis operand for `h.label ?: ...`")
	}
	if !ok || rt == nil {
		t.Fatal("member access on a proven in-file non-null property must resolve")
	}
	if rt.IsNullable() {
		t.Fatal("a non-null in-file property must not resolve as nullable")
	}
}

// TestUselessElvisResolvedOperandType_LiteralKeepsDirectPath ensures the
// gating change does not disturb intrinsically non-null operand shapes:
// a string literal still resolves via the direct resolver path.
func TestUselessElvisResolvedOperandType_LiteralKeepsDirectPath(t *testing.T) {
	code := "fun f(): String { val y = \"always\" ?: \"dead\"; return y }\n"
	rt, ok, present := resolveFirstElvisOperandType(t, code)
	if !present {
		t.Fatal("expected to find an elvis operand for `\"always\" ?: ...`")
	}
	if !ok || rt == nil {
		t.Fatal("a string literal operand must resolve via the direct path")
	}
	if rt.IsNullable() {
		t.Fatal("a string literal must not resolve as nullable")
	}
}
