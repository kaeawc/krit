package rules

import (
	"path/filepath"
	"regexp"
	"strings"

	"github.com/kaeawc/krit/internal/android"
	"github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// MissingInflatedIDRule mirrors Lint's MissingInflatedId: findViewById for an id the inflated layout never declares.
type MissingInflatedIDRule struct {
	FlatDispatchBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence: the inflated layout is found by syntax, not data flow.
func (r *MissingInflatedIDRule) Confidence() float64 { return api.ConfidenceMedium }

var layoutResourceRefRe = regexp.MustCompile(`(?:^|[^\w.]|\.)R\.(layout|id)\.(\w+)$`)

// appResourceRef returns the name in an app `R.<kind>.name` reference; framework `android.R` references do not count.
func appResourceRef(file *scanner.File, expr uint32, kind string) (string, bool) {
	if expr == 0 {
		return "", false
	}
	text := strings.TrimSpace(file.FlatNodeText(expr))
	if strings.HasPrefix(text, "android.R.") {
		return "", false
	}
	m := layoutResourceRefRe.FindStringSubmatch(text)
	if m == nil || m[1] != kind {
		return "", false
	}
	return m[2], true
}

// layoutIDs maps layout name to whether some variant of it declares ids; ids is the union, and includes marks layouts
// that contain an <include>, whose ids Lint cannot see.
type layoutIDInfo struct {
	ids      map[string]bool
	includes bool
}

func collectLayoutIDs(idx *android.ResourceIndex, only string) map[string]*layoutIDInfo {
	out := map[string]*layoutIDInfo{}
	for _, layout := range idx.LayoutVariants {
		if layout == nil || layout.RootView == nil {
			continue
		}
		name := strings.TrimSuffix(filepath.Base(layout.FilePath), filepath.Ext(layout.FilePath))
		if only != "" && name != only {
			continue
		}
		info := out[name]
		if info == nil {
			info = &layoutIDInfo{ids: map[string]bool{}}
			out[name] = info
		}
		var walk func(v *android.View)
		walk = func(v *android.View) {
			if v.Type == "include" {
				info.includes = true
			}
			if id := v.ID; id != "" {
				if i := strings.LastIndex(id, "/"); i >= 0 && (strings.HasPrefix(id, "@+id/") || strings.HasPrefix(id, "@id/")) {
					info.ids[id[i+1:]] = true
				}
			}
			for _, c := range v.Children {
				walk(c)
			}
		}
		walk(layout.RootView)
	}
	return out
}

func (r *MissingInflatedIDRule) check(ctx *api.Context) {
	file := ctx.File
	call, ok := lintCallAt(file, ctx.Idx)
	if !ok || (call.Name != "findViewById" && call.Name != "requireViewById") || len(call.Args) == 0 {
		return
	}
	id, ok := appResourceRef(file, call.Args[0], "id")
	if !ok {
		return
	}
	layout := inflatedLayoutFor(file, call)
	if layout == "" || ctx.ResourceIndex == nil {
		return
	}
	info := collectLayoutIDs(ctx.ResourceIndex, layout)[layout]
	if info == nil || info.includes || info.ids[id] {
		return
	}
	ctx.Emit(lintFindingAtNode(ctx, call.Args[0], "", "`@layout/"+layout+"` does not contain a declaration with id `"+id+"`"))
}

// inflatedLayoutFor finds the layout a findViewById call reads from: the layout inflated into the receiver variable
// (Java only, as in Lint), or the first unqualified setContentView in the enclosing block.
func inflatedLayoutFor(file *scanner.File, call lintCall) string {
	if call.ReceiverNode != 0 {
		if file.Language != scanner.LangJava || file.FlatType(call.ReceiverNode) != "identifier" {
			return ""
		}
		init := lintLocalVariableInitializer(file, call.Idx, strings.TrimSpace(call.ReceiverText))
		if init == 0 {
			return ""
		}
		inflate, ok := lintCallAt(file, init)
		if !ok || inflate.Name != "inflate" || len(inflate.Args) == 0 {
			return ""
		}
		name, _ := appResourceRef(file, inflate.Args[0], "layout")
		return name
	}
	block, ok := flatEnclosingAncestor(file, call.Idx, "block", "statements")
	if !ok {
		return ""
	}
	for stmt := file.FlatFirstChild(block); stmt != 0; stmt = file.FlatNextSib(stmt) {
		if !file.FlatIsNamed(stmt) {
			continue
		}
		expr := stmt
		if file.FlatType(stmt) == "expression_statement" {
			expr = file.FlatNamedChild(stmt, 0)
		}
		c, ok := lintCallAt(file, expr)
		if !ok || c.Name != "setContentView" || c.ReceiverNode != 0 || len(c.Args) == 0 {
			continue
		}
		name, _ := appResourceRef(file, c.Args[0], "layout")
		return name
	}
	return ""
}

// lintLocalVariableInitializer returns the initializer of a local variable (not a parameter or field) called name.
func lintLocalVariableInitializer(file *scanner.File, scope uint32, name string) uint32 {
	fn, ok := flatEnclosingCallable(file, scope)
	if !ok {
		return 0
	}
	var init uint32
	file.FlatWalkNodes(fn, "variable_declarator", func(decl uint32) {
		if id, ok := file.FlatFindChild(decl, "identifier"); ok && file.FlatNodeText(id) == name {
			if parent, _ := file.FlatParent(decl); file.FlatType(parent) == "local_variable_declaration" {
				init = lintInitializerAfterEquals(file, decl)
			}
		}
	})
	return init
}
