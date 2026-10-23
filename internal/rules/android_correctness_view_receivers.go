package rules

import (
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
	"github.com/kaeawc/krit/internal/typeinfer"
)

// WrongCallRule detects calling the wrong View draw/layout methods.
type WrongCallRule struct {
	FlatDispatchBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence. This is an
// Android-lint port from AOSP; the detection relies on source-text
// patterns (call names, string literal contents, hardcoded allow-
// lists of API names) rather than type resolution, so project-
// specific wrapper APIs can cause false positives or negatives.
// Classified per roadmap/17.
func (r *WrongCallRule) Confidence() float64 { return api.ConfidenceMedium }

// Remaining rules are in android_correctness_checks.go

// ---------------------------------------------------------------------------
// init() -- register first set of correctness rules
// ---------------------------------------------------------------------------

// Remaining correctness rules are in android_correctness_checks.go

// ---------------------------------------------------------------------------
// View hierarchy receiver-proof helpers (used by SetTextI18n, WrongCall, ...)
// ---------------------------------------------------------------------------

// androidNonViewReceiverRoots names top-level symbols whose builder/widget
// chains expose setText / onDraw / onMeasure / onLayout methods but are
// NOT android.view.View subtypes. A receiver chain starting with one of
// these names must NOT be flagged by View-targeted rules. Toolbar,
// TabLayout, Snackbar, etc. are deliberately omitted — they ARE View
// subclasses and `view.setText(...)` should still fire.
var androidNonViewReceiverRoots = map[string]struct{}{
	"NotificationCompat":     {},
	"Notification":           {},
	"RemoteViews":            {},
	"AccessibilityNodeInfo":  {},
	"AccessibilityRecord":    {},
	"AccessibilityEvent":     {},
	"SpannableStringBuilder": {},
	"Spannable":              {},
	"PrintAttributes":        {},
	"MediaSessionCompat":     {},
	"MenuItem":               {},
	"MenuItemCompat":         {},
	"ActionBar":              {},
	"AlertDialog":            {},
	"AlertDialogCompat":      {},
	"Preference":             {},
	"PreferenceFragment":     {},
	"Tab":                    {},
}

// androidReceiverChainRoot returns the leftmost identifier of the receiver
// chain reaching `nav`. For `NotificationCompat.Builder(ctx).setText(...)`
// the navigation receiver is the call `NotificationCompat.Builder(ctx)`
// and we want to return "NotificationCompat". Returns "" if the root is
// not a simple identifier (e.g. a function-call result or `this`).
func androidReceiverChainRoot(file *scanner.File, recv uint32) string {
	if file == nil || recv == 0 {
		return ""
	}
	for depth := 0; depth < 32 && recv != 0; depth++ {
		switch file.FlatType(recv) {
		case "simple_identifier", "type_identifier":
			return file.FlatNodeText(recv)
		case "navigation_expression":
			// receiver is leftmost named child of navigation_expression
			next := uint32(0)
			for c := file.FlatFirstChild(recv); c != 0; c = file.FlatNextSib(c) {
				if file.FlatIsNamed(c) {
					next = c
					break
				}
			}
			if next == 0 || next == recv {
				return ""
			}
			recv = next
		case "call_expression":
			// recurse into the callee
			next := uint32(0)
			for c := file.FlatFirstChild(recv); c != 0; c = file.FlatNextSib(c) {
				if file.FlatIsNamed(c) {
					next = c
					break
				}
			}
			if next == 0 || next == recv {
				return ""
			}
			recv = next
		case "parenthesized_expression":
			next := uint32(0)
			for c := file.FlatFirstChild(recv); c != 0; c = file.FlatNextSib(c) {
				if file.FlatIsNamed(c) {
					next = c
					break
				}
			}
			if next == 0 || next == recv {
				return ""
			}
			recv = next
		case "this_expression":
			return "this"
		case "super_expression":
			return "super"
		default:
			return ""
		}
	}
	return ""
}

// androidReceiverIsKnownNonView reports whether the leftmost identifier of
// the receiver chain is a documented non-View symbol whose methods include
// setText / onDraw / onMeasure / onLayout but are NOT View members. Used
// to suppress View-targeted rules on builder / RemoteViews / accessibility
// chains.
func androidReceiverIsKnownNonView(file *scanner.File, recv uint32) bool {
	root := androidReceiverChainRoot(file, recv)
	if root == "" {
		return false
	}
	_, present := androidNonViewReceiverRoots[root]
	return present
}

// androidTypeIsViewSubtype reports whether the resolved type is android.view.View
// or a known subtype. Walks supertypes via the resolver to handle source-declared
// View subclasses too. The resolver may be nil — in that case only the type's
// own simple/FQN names are checked against the View name.
func androidTypeIsViewSubtype(resolver typeinfer.TypeResolver, typ *typeinfer.ResolvedType) bool {
	if typ == nil {
		return false
	}
	seen := make(map[string]bool)
	var visit func(string) bool
	visit = func(name string) bool {
		if name == "" || seen[name] {
			return false
		}
		seen[name] = true
		if name == "View" || name == "android.view.View" {
			return true
		}
		if resolver == nil {
			return false
		}
		info := resolver.ClassHierarchy(name)
		if info == nil {
			return false
		}
		if info.Name == "View" || info.FQN == "android.view.View" {
			return true
		}
		for _, supertype := range info.Supertypes {
			if visit(supertype) {
				return true
			}
		}
		return false
	}
	return visit(typ.FQN) || visit(typ.Name)
}

// androidTypeIsTextViewSubtype reports whether the resolved type is
// android.widget.TextView or a known subtype (Button, EditText, ...).
// Walks supertypes via the resolver to handle source-declared TextView
// subclasses too.
func androidTypeIsTextViewSubtype(resolver typeinfer.TypeResolver, typ *typeinfer.ResolvedType) bool {
	if typ == nil {
		return false
	}
	seen := make(map[string]bool)
	var visit func(string) bool
	visit = func(name string) bool {
		if name == "" || seen[name] {
			return false
		}
		seen[name] = true
		if name == "TextView" || name == "android.widget.TextView" {
			return true
		}
		if resolver == nil {
			return false
		}
		info := resolver.ClassHierarchy(name)
		if info == nil {
			return false
		}
		if info.Name == "TextView" || info.FQN == "android.widget.TextView" {
			return true
		}
		for _, supertype := range info.Supertypes {
			if visit(supertype) {
				return true
			}
		}
		return false
	}
	return visit(typ.FQN) || visit(typ.Name)
}

// androidEnclosingClassExtendsView reports whether the class_declaration
// enclosing `idx` extends android.view.View or a known View subtype. Walks
// declared supertypes and asks the resolver to follow ancestors.
func androidEnclosingClassExtendsView(file *scanner.File, idx uint32, resolver typeinfer.TypeResolver) bool {
	if file == nil || idx == 0 {
		return false
	}
	classIdx, ok := flatEnclosingAncestor(file, idx, "class_declaration", "object_declaration")
	if !ok || classIdx == 0 {
		return false
	}
	for _, super := range androidDirectSupertypesFlat(file, classIdx) {
		if super.name == "" {
			continue
		}
		if androidSupertypeIsView(super, file, resolver) {
			return true
		}
	}
	return false
}

func androidSupertypeIsView(super androidSupertypeRef, file *scanner.File, resolver typeinfer.TypeResolver) bool {
	// Fast path: the supertype is literally View / qualified android.view.View.
	if super.simple == "View" || super.name == "android.view.View" {
		return true
	}
	if resolver == nil {
		// Without a resolver we can still recognise the common simple names.
		if _, ok := androidKnownViewSimpleNames[super.simple]; ok {
			return true
		}
		return false
	}
	// Resolve the imported FQN first, then walk class hierarchy.
	fqn := super.name
	if !super.qualified {
		if resolved := resolver.ResolveImport(super.simple, file); resolved != "" {
			fqn = resolved
		}
	}
	candidate := &typeinfer.ResolvedType{Name: super.simple, FQN: fqn, Kind: typeinfer.TypeClass}
	if androidTypeIsViewSubtype(resolver, candidate) {
		return true
	}
	// Last resort: the simple-name allowlist (covers cases where the
	// resolver lacks hierarchy info for a framework type).
	if _, ok := androidKnownViewSimpleNames[super.simple]; ok {
		return true
	}
	return false
}

// androidKnownViewSimpleNames is the simple-name fallback for cases where
// the resolver cannot follow the hierarchy. Conservative — only includes
// the framework / AndroidX View types most likely to be subclassed by
// custom views overriding onDraw / onMeasure / onLayout.
var androidKnownViewSimpleNames = map[string]struct{}{
	"View":                 {},
	"ViewGroup":            {},
	"TextView":             {},
	"AppCompatTextView":    {},
	"EditText":             {},
	"AppCompatEditText":    {},
	"Button":               {},
	"AppCompatButton":      {},
	"MaterialButton":       {},
	"ImageView":            {},
	"AppCompatImageView":   {},
	"ShapeableImageView":   {},
	"FrameLayout":          {},
	"LinearLayout":         {},
	"RelativeLayout":       {},
	"ConstraintLayout":     {},
	"MotionLayout":         {},
	"CoordinatorLayout":    {},
	"SurfaceView":          {},
	"TextureView":          {},
	"RecyclerView":         {},
	"ListView":             {},
	"GridView":             {},
	"ScrollView":           {},
	"NestedScrollView":     {},
	"HorizontalScrollView": {},
	"WebView":              {},
	"CardView":             {},
	"AbstractComposeView":  {},
	"ComposeView":          {},
	"FloatingActionButton": {},
	"MaterialCardView":     {},
	"AppCompatCheckBox":    {},
	"AppCompatRadioButton": {},
	"SwitchCompat":         {},
	"TextInputEditText":    {},
}
