package rules

import (
	"strings"

	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/rules/semantics"
	"github.com/kaeawc/krit/internal/scanner"
	"github.com/kaeawc/krit/internal/typeinfer"
)

type ViewTagRule struct {
	FlatDispatchBase
	AndroidRule
}

func (r *ViewTagRule) NodeTypes() []string { return []string{"call_expression"} }

// Confidence reports a high-confidence semantic check. The rule now anchors
// on setTag call expressions, verifies the receiver is a View, and classifies
// the tagged value by resolved type instead of variable names.
func (r *ViewTagRule) Confidence() float64 { return api.ConfidenceHigh }

var viewTagReceiverTypes = []string{
	"android.view.View",
}

var viewTagFrameworkHeavyTypes = []string{
	"android.app.Activity",
	"android.app.Fragment",
	"android.content.Context",
	"android.database.Cursor",
	"android.graphics.Bitmap",
	"android.graphics.drawable.Drawable",
	"android.support.v4.app.Fragment",
	"android.support.v7.widget.RecyclerView.Adapter",
	"android.view.View",
	"android.widget.Adapter",
	"android.widget.ListAdapter",
	"androidx.fragment.app.Fragment",
	"androidx.recyclerview.widget.RecyclerView.Adapter",
}

const viewTagLeakMessage = "View.setTag() with a framework object may cause memory leaks across configuration changes."

func (r *ViewTagRule) check(ctx *api.Context) {
	if ctx.File.FlatType(ctx.Idx) != "call_expression" {
		return
	}
	target, ok := semantics.ResolveCallTarget(ctx, ctx.Idx)
	if !ok || target.CalleeName != "setTag" || len(target.Arguments) != 1 {
		return
	}
	confidence, ok := viewTagReceiverEvidence(ctx, target)
	if !ok {
		return
	}
	arg := target.Arguments[0]
	if !arg.Valid() || !viewTagArgumentIsFrameworkHeavy(ctx, arg.Node) {
		return
	}
	ctx.Emit(scanner.Finding{
		Line:       ctx.File.FlatRow(ctx.Idx) + 1,
		Col:        ctx.File.FlatCol(ctx.Idx) + 1,
		Message:    viewTagLeakMessage,
		Confidence: confidence,
	})
}

func viewTagReceiverEvidence(ctx *api.Context, target semantics.CallTarget) (float64, bool) {
	if target.Resolved {
		if viewTagCallTargetIsViewSetTag(target.QualifiedName) {
			return 0.85, true
		}
		return 0, false
	}
	if !target.Receiver.Valid() {
		return 0, false
	}
	typ, ok := semantics.ExpressionType(ctx, target.Receiver.Node)
	if !ok {
		return 0, false
	}
	if viewTagFrameworkTypeMatches(ctx, typ.Type, viewTagReceiverTypes) {
		return 0.85, true
	}
	if viewTagSameFileClassTypeMatches(ctx, typ.Type, viewTagReceiverTypes) {
		return 0.80, true
	}
	return 0, false
}

func viewTagArgumentIsFrameworkHeavy(ctx *api.Context, expr uint32) bool {
	typ, ok := semantics.ExpressionType(ctx, expr)
	if !ok {
		return false
	}
	if viewTagTypeMatches(ctx, typ.Type, viewTagFrameworkHeavyTypes) {
		return true
	}
	return viewTagTypeNameSuggestsHolder(typ.Type)
}

// viewTagTypeNameSuggestsHolder applies the AOSP ViewTagDetector heuristic:
// a class whose simple name contains "Holder" or "Tag" is presumed to hold
// view references (e.g. a ViewHolder pattern), making it a leak risk on
// API < 14 where View.setTag() is backed by a static map.
func viewTagTypeNameSuggestsHolder(typ *typeinfer.ResolvedType) bool {
	if typ == nil || typ.Kind == typeinfer.TypeUnknown || typ.Kind == typeinfer.TypePrimitive {
		return false
	}
	name := typ.Name
	if name == "" {
		name = androidSimpleName(typ.FQN)
	}
	if name == "" {
		return false
	}
	return strings.Contains(name, "Holder") || strings.Contains(name, "Tag")
}

func viewTagCallTargetIsViewSetTag(target string) bool {
	target = strings.TrimSpace(target)
	if cut := strings.Index(target, "("); cut >= 0 {
		target = target[:cut]
	}
	for _, suffix := range []string{".setTag", "#setTag"} {
		if !strings.HasSuffix(target, suffix) {
			continue
		}
		owner := strings.TrimSuffix(target, suffix)
		return viewTagFQNMatchesAny(owner, viewTagReceiverTypes) ||
			viewTagKnownSubtypeOfAny(owner, viewTagReceiverTypes)
	}
	return false
}

func viewTagFrameworkTypeMatches(ctx *api.Context, typ *typeinfer.ResolvedType, targets []string) bool {
	if typ == nil || typ.Kind == typeinfer.TypeUnknown {
		return false
	}
	if viewTagFQNMatchesAny(typ.FQN, targets) || viewTagKnownSubtypeOfAny(typ.FQN, targets) {
		return true
	}
	if ctx.Resolver == nil || ctx.File == nil || typ.Name == "" {
		return false
	}
	if fqn := ctx.Resolver.ResolveImport(typ.Name, ctx.File); fqn != "" {
		return viewTagFQNMatchesAny(fqn, targets) || viewTagKnownSubtypeOfAny(fqn, targets)
	}
	return false
}

func viewTagSameFileClassTypeMatches(ctx *api.Context, typ *typeinfer.ResolvedType, targets []string) bool {
	if ctx.Resolver == nil || ctx.File == nil || typ == nil || typ.Kind == typeinfer.TypeUnknown {
		return false
	}
	for _, candidate := range []string{typ.FQN, typ.Name} {
		if candidate == "" {
			continue
		}
		info := ctx.Resolver.ClassHierarchy(candidate)
		if info == nil || info.File != ctx.File.Path {
			continue
		}
		if viewTagClassInfoMatches(ctx, info, targets, map[string]bool{}) {
			return true
		}
	}
	return false
}

func viewTagTypeMatches(ctx *api.Context, typ *typeinfer.ResolvedType, targets []string) bool {
	if typ == nil || typ.Kind == typeinfer.TypeUnknown {
		return false
	}
	if viewTagResolvedTypeMatches(typ, targets) {
		return true
	}
	if ctx.Resolver == nil || ctx.File == nil {
		return false
	}
	if typ.FQN != "" {
		if viewTagClassHierarchyMatches(ctx, typ.FQN, targets, nil) {
			return true
		}
	}
	if typ.Name != "" {
		if fqn := ctx.Resolver.ResolveImport(typ.Name, ctx.File); fqn != "" {
			return viewTagFQNMatchesAny(fqn, targets) ||
				viewTagKnownSubtypeOfAny(fqn, targets) ||
				viewTagClassHierarchyMatches(ctx, fqn, targets, nil)
		}
		if info := ctx.Resolver.ClassHierarchy(typ.Name); info != nil && info.File != "" {
			return viewTagClassInfoMatches(ctx, info, targets, map[string]bool{})
		}
	}
	return false
}

func viewTagResolvedTypeMatches(typ *typeinfer.ResolvedType, targets []string) bool {
	if typ == nil {
		return false
	}
	if viewTagFQNMatchesAny(typ.FQN, targets) || viewTagKnownSubtypeOfAny(typ.FQN, targets) {
		return true
	}
	for _, super := range typ.Supertypes {
		if viewTagFQNMatchesAny(super, targets) || viewTagKnownSubtypeOfAny(super, targets) {
			return true
		}
	}
	return false
}

func viewTagClassHierarchyMatches(ctx *api.Context, typeName string, targets []string, visited map[string]bool) bool {
	if ctx.Resolver == nil || typeName == "" {
		return false
	}
	info := ctx.Resolver.ClassHierarchy(typeName)
	if info == nil {
		return false
	}
	if visited == nil {
		visited = map[string]bool{}
	}
	return viewTagClassInfoMatches(ctx, info, targets, visited)
}

func viewTagClassInfoMatches(ctx *api.Context, info *typeinfer.ClassInfo, targets []string, visited map[string]bool) bool {
	if info == nil {
		return false
	}
	key := info.FQN
	if key == "" {
		key = info.Name
	}
	if key != "" {
		if visited[key] {
			return false
		}
		visited[key] = true
	}
	if viewTagFQNMatchesAny(info.FQN, targets) || viewTagKnownSubtypeOfAny(info.FQN, targets) {
		return true
	}
	for _, super := range info.Supertypes {
		if viewTagFQNMatchesAny(super, targets) || viewTagKnownSubtypeOfAny(super, targets) {
			return true
		}
		if viewTagClassHierarchyMatches(ctx, super, targets, visited) {
			return true
		}
	}
	return false
}

func viewTagFQNMatchesAny(fqn string, targets []string) bool {
	if fqn == "" {
		return false
	}
	for _, target := range targets {
		if fqn == target {
			return true
		}
	}
	return false
}

func viewTagKnownSubtypeOfAny(fqn string, targets []string) bool {
	if fqn == "" {
		return false
	}
	for _, target := range targets {
		if typeinfer.IsKnownSubtype(fqn, target) {
			return true
		}
	}
	return false
}
