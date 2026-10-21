package rules

import (
	"bytes"
	"strings"

	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

func classExtendsAnyFlat(file *scanner.File, class uint32, names ...string) bool {
	for child := file.FlatFirstChild(class); child != 0; child = file.FlatNextSib(child) {
		switch file.FlatType(child) {
		case "delegation_specifier":
			typeName := viewConstructorSupertypeNameFlat(file, child)
			if semanticTypeNameMatches(typeName, names...) {
				return true
			}
		case "superclass", "super_interfaces":
			if javaTypeContainerMatchesAny(file, child, names...) {
				return true
			}
		}
	}
	return false
}

func javaTypeContainerMatchesAny(file *scanner.File, idx uint32, names ...string) bool {
	if file == nil || idx == 0 {
		return false
	}
	if matchTypeBytes(file.FlatNodeBytes(idx), names) {
		return true
	}
	found := false
	file.FlatWalkAllNodes(idx, func(child uint32) {
		if found {
			return
		}
		switch file.FlatType(child) {
		case "type_identifier", "scoped_type_identifier", "scoped_identifier", "generic_type":
			if matchTypeBytes(file.FlatNodeBytes(child), names) {
				found = true
			}
		}
	})
	return found
}

// matchTypeBytes is the byte-level equivalent of the
// "semanticTypeNameMatches(s, name) || strings.Contains(s, name)"
// fallback chain used by the Java supertype walker. The fast path is a
// pure bytes.Contains; the $-form normalization (only meaningful for
// nested-class JNI-style names) is paid for only when an unambiguous
// '$' is present, which keeps the common case allocation-free.
func matchTypeBytes(text []byte, names []string) bool {
	text = bytes.TrimSpace(text)
	if len(text) == 0 {
		return false
	}
	for _, name := range names {
		if bytes.Contains(text, []byte(name)) {
			return true
		}
	}
	// $→. normalization can rescue a match only when either side carries
	// '$'. Today no caller passes a $-form name, but guarding both keeps
	// the fast path strictly equivalent to the prior implementation.
	hasDollar := bytes.IndexByte(text, '$') >= 0
	if !hasDollar {
		for _, name := range names {
			if strings.IndexByte(name, '$') >= 0 {
				hasDollar = true
				break
			}
		}
	}
	if !hasDollar {
		return false
	}
	s := string(text)
	for _, name := range names {
		if semanticTypeNameMatches(s, name) {
			return true
		}
	}
	return false
}

func classHasMemberFunctionFlat(file *scanner.File, class uint32, name string) bool {
	body, _ := file.FlatFindChild(class, "class_body")
	if body == 0 {
		return false
	}
	found := false
	file.FlatWalkNodes(body, "function_declaration", func(fn uint32) {
		if found {
			return
		}
		if extractIdentifierFlat(file, fn) == name {
			found = true
		}
	})
	file.FlatWalkNodes(body, "method_declaration", func(fn uint32) {
		if found {
			return
		}
		if extractIdentifierFlat(file, fn) == name {
			found = true
		}
	})
	return found
}

func classHasNestedViewHolderFlat(file *scanner.File, class uint32) bool {
	body, _ := file.FlatFindChild(class, "class_body")
	if body == 0 {
		return false
	}
	found := false
	file.FlatWalkNodes(body, "class_declaration", func(child uint32) {
		if found || child == class {
			return
		}
		name := extractIdentifierFlat(file, child)
		if strings.Contains(name, "ViewHolder") || classExtendsAnyFlat(file, child, "RecyclerView.ViewHolder", "ViewHolder") {
			found = true
		}
	})
	return found
}

// recyclerAdapterSupertypeNames are the supertype names the RecyclerAdapter
// rules treat as Adapter-implementing. Shared between the local AST check and
// the resolver-fallback hierarchy walk to keep both paths in sync.
var recyclerAdapterSupertypeNames = []string{
	"androidx.recyclerview.widget.RecyclerView.Adapter",
	"android.support.v7.widget.RecyclerView.Adapter",
	"android.widget.BaseAdapter",
	"RecyclerView.Adapter",
	"BaseAdapter",
	"Adapter",
}

// lastTypeNameSegment returns the trailing simple-name segment of a (possibly
// qualified or nested) type name, splitting on both '.' and '$'.
func lastTypeNameSegment(name string) string {
	name = strings.TrimSpace(name)
	if i := strings.LastIndexAny(name, ".$"); i >= 0 {
		name = name[i+1:]
	}
	if i := strings.IndexByte(name, '<'); i >= 0 {
		name = name[:i]
	}
	return strings.TrimSpace(name)
}

// simpleTypeNameIsAdapter reports whether a supertype's *simple* name is an
// actual RecyclerView/list adapter rather than a lookalike that merely
// contains the substring "Adapter" (e.g. RecyclerView.AdapterDataObserver).
// The bare "Adapter" entry in recyclerAdapterSupertypeNames is matched via a
// substring contains in the Java supertype walker, which is what lets a
// `...AdapterDataObserver` supertype slip through; this suffix-boundary check
// is the corrective gate.
func simpleTypeNameIsAdapter(simple string) bool {
	if simple == "" {
		return false
	}
	return strings.HasSuffix(simple, "Adapter")
}

// classDirectSupertypeIsAdapterFlat confirms at least one *direct* supertype of
// the class has a simple name ending in "Adapter". It deliberately re-walks the
// direct delegation/superclass nodes (instead of trusting the contains-based
// gate) so observer/listener siblings that byte-contain "Adapter" are excluded.
func classDirectSupertypeIsAdapterFlat(file *scanner.File, class uint32) bool {
	if file == nil || class == 0 {
		return false
	}
	found := false
	for child := file.FlatFirstChild(class); child != 0 && !found; child = file.FlatNextSib(child) {
		switch file.FlatType(child) {
		case "delegation_specifier":
			if name := viewConstructorSupertypeNameFlat(file, child); simpleTypeNameIsAdapter(name) {
				found = true
			}
		case "superclass", "super_interfaces":
			file.FlatWalkAllNodes(child, func(n uint32) {
				if found {
					return
				}
				switch file.FlatType(n) {
				case "type_identifier", "scoped_type_identifier", "scoped_identifier", "generic_type":
					if simpleTypeNameIsAdapter(lastTypeNameSegment(file.FlatNodeText(n))) {
						found = true
					}
				}
			})
		}
	}
	return found
}

func isRecyclerAdapterClassFlat(ctx *api.Context, class uint32) bool {
	file := ctx.File
	if classExtendsAnyFlat(file, class, recyclerAdapterSupertypeNames...) &&
		classDirectSupertypeIsAdapterFlat(file, class) {
		return true
	}
	if ctx.Resolver == nil || !classHasAdapterLikeSupertypeFlat(file, class) {
		return false
	}
	info := ctx.Resolver.ClassHierarchy(extractIdentifierFlat(file, class))
	if info == nil {
		return false
	}
	for _, st := range info.Supertypes {
		if semanticTypeNameMatches(st, recyclerAdapterSupertypeNames...) &&
			simpleTypeNameIsAdapter(lastTypeNameSegment(st)) {
			return true
		}
	}
	return false
}

// classHasAdapterLikeSupertypeFlat returns true when the class has at least
// one direct supertype whose type identifier ends in "Adapter" or contains
// "RecyclerView" — a cheap signal that consulting the type oracle might
// produce a real Adapter match. The walk is restricted to type-identifier
// shapes so generic args, lambda bodies, and value arguments don't trigger.
func classHasAdapterLikeSupertypeFlat(file *scanner.File, class uint32) bool {
	if file == nil || class == 0 {
		return false
	}
	adapter := []byte("Adapter")
	recyclerView := []byte("RecyclerView")
	found := false
	for child := file.FlatFirstChild(class); child != 0 && !found; child = file.FlatNextSib(child) {
		switch file.FlatType(child) {
		case "delegation_specifier", "superclass", "super_interfaces":
		default:
			continue
		}
		file.FlatWalkAllNodes(child, func(n uint32) {
			if found {
				return
			}
			switch file.FlatType(n) {
			case "type_identifier", "scoped_type_identifier", "scoped_identifier", "generic_type":
				text := file.FlatNodeBytes(n)
				if bytes.HasSuffix(text, adapter) || bytes.Contains(text, recyclerView) {
					found = true
				}
			}
		})
	}
	return found
}
