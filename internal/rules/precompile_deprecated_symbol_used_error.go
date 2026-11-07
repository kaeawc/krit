package rules

import (
	"fmt"

	"github.com/kaeawc/krit/internal/filefacts"
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// PrecompileDeprecatedSymbolUsedErrorRule flags references to symbols
// whose @Deprecated annotation specifies level = DeprecationLevel.ERROR.
// Mirrors kotlinc's DEPRECATION_ERROR.
//
// Library symbols are not yet classified by level: the oracle exposes
// kotlin.Deprecated FQNs but no annotation arguments, and firing on
// every @Deprecated would flood ordinary (default WARNING) call sites.
// The rule declares NeedsOracleMemberAnnotations so the oracle profile
// is already sized for the library path once the JVM tools surface the
// `level` argument.
type PrecompileDeprecatedSymbolUsedErrorRule struct {
	FlatDispatchBase
	BaseRule
}

func (r *PrecompileDeprecatedSymbolUsedErrorRule) Confidence() float64 { return api.ConfidenceVeryHigh }

func (r *PrecompileDeprecatedSymbolUsedErrorRule) check(ctx *api.Context) {
	idx, file := ctx.Idx, ctx.File

	if len(deprecatedDeclIndex(file)) == 0 {
		return
	}

	nodeType := file.FlatType(idx)
	if file.FlatHasAncestorOfType(idx, "import_header") {
		return
	}
	if nodeType == "navigation_expression" {
		if parent, ok := file.FlatParent(idx); ok && file.FlatType(parent) == "call_expression" {
			return
		}
	}
	if nodeType == "user_type" && file.FlatHasAncestorOfType(idx, "annotation") {
		return
	}

	name := flatDeprecationRefName(file, idx)
	if name == "" {
		return
	}

	// The reference must resolve to one declaration that is deprecated with level ERROR, not to an
	// overload or to a same-named member of another class.
	site := deprecatedReferenceTarget(ctx, idx, name)
	if site == nil || site.info == nil || site.info.level != "ERROR" {
		return
	}
	info := site.info

	msg := fmt.Sprintf("'%s' is deprecated with level ERROR and must not be used.", name)
	if info.message != "" {
		msg = fmt.Sprintf("'%s' is deprecated with level ERROR: %s", name, info.message)
	}
	ctx.EmitAt(file.FlatRow(idx)+1, file.FlatCol(idx)+1, msg)
}

// precompileDeclSite is one same-file declaration of a name.
type precompileDeclSite struct {
	owner       string // nearest enclosing class or object name, "" for top level
	isExtension bool
	info        *deprecationInfo // non-nil when the declaration carries @Deprecated
}

// precompileDeclSites indexes every function, class, object and property declaration in the file by name.
func precompileDeclSites(file *scanner.File) map[string][]precompileDeclSite {
	return filefacts.FileFact(fileFactsCache(), file, slotPrecompileDeclSites, func() map[string][]precompileDeclSite {
		out := make(map[string][]precompileDeclSite)
		file.FlatWalkAllNodes(0, func(idx uint32) {
			switch file.FlatType(idx) {
			case "function_declaration", "class_declaration", "object_declaration", "property_declaration":
			default:
				return
			}
			name := extractIdentifierFlat(file, idx)
			if name == "" {
				return
			}
			owner := ""
			if cls, ok := flatEnclosingAncestor(file, idx, "class_declaration", "object_declaration"); ok {
				owner = extractIdentifierFlat(file, cls)
			}
			out[name] = append(out[name], precompileDeclSite{
				owner:       owner,
				isExtension: duplicateDeclReceiver(file, idx) != "",
				info:        extractDeprecatedInfoFlat(file, idx),
			})
		})
		return out
	})
}

// deprecatedReferenceTarget returns the single same-file declaration a reference to name can only be, or nil
// when the target is unknown: overloads in one owner, receivers of unknown type, or declarations elsewhere.
func deprecatedReferenceTarget(ctx *api.Context, idx uint32, name string) *precompileDeclSite {
	file := ctx.File
	sites := precompileDeclSites(file)[name]
	if len(sites) == 0 {
		return nil
	}
	only := func(owner string) *precompileDeclSite {
		var match *precompileDeclSite
		for i := range sites {
			if sites[i].owner != owner {
				continue
			}
			if match != nil {
				return nil
			}
			match = &sites[i]
		}
		return match
	}
	navExpr := uint32(0)
	switch file.FlatType(idx) {
	case "navigation_expression":
		navExpr = idx
	case "call_expression":
		if callee := file.FlatFirstChild(idx); callee != 0 && file.FlatType(callee) == "navigation_expression" {
			navExpr = callee
		}
	}
	if navExpr == 0 {
		// Unqualified: the innermost enclosing class that declares the name wins, then the top level.
		for cls, ok := flatEnclosingAncestor(file, idx, "class_declaration", "object_declaration"); ok; cls, ok = flatEnclosingAncestor(file, cls, "class_declaration", "object_declaration") {
			owner := extractIdentifierFlat(file, cls)
			for i := range sites {
				if sites[i].owner == owner {
					return only(owner)
				}
			}
		}
		return only("")
	}
	receiver := file.FlatFirstChild(navExpr)
	if receiver == 0 {
		return nil
	}
	for _, s := range sites {
		if s.owner != "" && receiverDeclaredWithType(file, receiver, s.owner) {
			return only(s.owner)
		}
	}
	return nil
}

// receiverDeclaredWithType reports a receiver that is `this` inside the named class, or a simple name whose
// parameter, constructor property or property declaration in scope has the named class as its explicit type.
func receiverDeclaredWithType(file *scanner.File, receiver uint32, owner string) bool {
	if file.FlatType(receiver) == "this_expression" {
		for cls, ok := flatEnclosingAncestor(file, receiver, "class_declaration", "object_declaration"); ok; cls, ok = flatEnclosingAncestor(file, cls, "class_declaration", "object_declaration") {
			if extractIdentifierFlat(file, cls) == owner {
				return true
			}
		}
		return false
	}
	if file.FlatType(receiver) != "simple_identifier" {
		return false
	}
	name := file.FlatNodeText(receiver)
	for scope, ok := file.FlatParent(receiver); ok; scope, ok = file.FlatParent(scope) {
		switch file.FlatType(scope) {
		case "function_declaration", "class_declaration", "object_declaration":
		default:
			continue
		}
		found, typed := false, false
		for _, nodeType := range []string{"parameter", "class_parameter", "property_declaration"} {
			file.FlatWalkNodes(scope, nodeType, func(decl uint32) {
				if found || extractIdentifierFlat(file, decl) != name {
					return
				}
				found = true
				typed = lastTypeSegment(declaredTypeText(file, decl)) == owner
			})
		}
		if found {
			return typed
		}
	}
	return false
}

func declaredTypeText(file *scanner.File, decl uint32) string {
	if t := extractParameterTypeNameFlat(file, decl); t != "" {
		return t
	}
	return extractPropertyTypeNameFlat(file, decl)
}
