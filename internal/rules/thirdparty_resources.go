package rules

// Ports of slack-lints' resource import-alias checks (slackhq/slack-lints 0.11.1,
// slack-lint-checks/src/main/java/slack/lint/resources/): FullyQualifiedResourceDetector,
// MissingResourceImportAliasDetector and WrongResourceImportAliasDetector. Import aliases are a Kotlin feature, so
// all three check Kotlin only. Each reads Lint's import-aliases option, here the importAliases list of
// `package.R as Alias` entries.

import (
	"bytes"
	"strings"

	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// ResourceImportAliasRule is the shared base of the three checks; ImportAliases is its importAliases option.
type ResourceImportAliasRule struct {
	FlatDispatchBase
	BaseRule
	ImportAliases []string
}

// FullyQualifiedResourceRule reports fully qualified R class references.
type FullyQualifiedResourceRule struct{ ResourceImportAliasRule }

// MissingResourceImportAliasRule reports R class imports from other modules without an alias.
type MissingResourceImportAliasRule struct{ ResourceImportAliasRule }

// WrongResourceImportAliasRule reports R class imports whose alias differs from the configured one.
type WrongResourceImportAliasRule struct{ ResourceImportAliasRule }

// Confidence reports a high base confidence: the checks read import directives and qualified names only.
func (r *ResourceImportAliasRule) Confidence() float64 { return api.ConfidenceHigh }

// aliases parses the option into R class FQN -> alias, as ImportAliasesLoader does.
func (r *ResourceImportAliasRule) aliases() map[string]string {
	out := map[string]string{}
	for _, entry := range r.ImportAliases {
		for _, part := range strings.Split(entry, ",") {
			pkg, alias, ok := strings.Cut(strings.TrimSpace(part), " as ")
			if ok && strings.TrimSpace(pkg) != "" {
				out[strings.TrimSpace(pkg)] = strings.TrimSpace(alias)
			}
		}
	}
	return out
}

func resourceRFileGate(file *scanner.File) bool { return bytes.Contains(file.Content, []byte(".R")) }

// checkFullyQualified reports a qualified reference whose receiver is an R class other than android.R.
func (r *ResourceImportAliasRule) checkFullyQualified(ctx *api.Context) {
	file, idx := ctx.File, ctx.Idx
	recv := file.FlatFirstChild(idx)
	if !resourceIsDottedName(file, recv) {
		return
	}
	qualifier := strings.Join(strings.Fields(file.FlatNodeText(recv)), "")
	if !strings.HasSuffix(qualifier, ".R") || qualifier == "android.R" {
		return
	}
	msg := "Use an import alias instead"
	if alias, ok := r.aliases()[qualifier]; ok {
		msg = "Use " + alias + " as an import alias instead"
	}
	ctx.EmitAtNode(recv, msg)
}

// resourceIsDottedName reports whether node is a qualified name `a.b.c` with no calls or other expressions.
func resourceIsDottedName(file *scanner.File, node uint32) bool {
	switch file.FlatType(node) {
	case "simple_identifier":
		return true
	case "navigation_expression":
		first := file.FlatFirstChild(node)
		suffix := file.FlatNextSib(first)
		id, ok := file.FlatFindChild(suffix, "simple_identifier")
		return ok && file.FlatNextSib(suffix) == 0 && file.FlatType(suffix) == "navigation_suffix" &&
			file.FlatNextSib(id) == 0 && resourceIsDottedName(file, first)
	}
	return false
}

// resourceImport is one `import a.b.R [as X]` directive.
type resourceImport struct {
	node  uint32
	fqn   string
	alias string
}

func resourceRImports(file *scanner.File, list uint32) []resourceImport {
	var out []resourceImport
	for header := file.FlatFirstChild(list); header != 0; header = file.FlatNextSib(header) {
		if file.FlatType(header) != "import_header" || file.FlatHasChildOfType(header, "wildcard_import") {
			continue
		}
		id, ok := file.FlatFindChild(header, "identifier")
		if !ok {
			continue
		}
		imp := resourceImport{node: header, fqn: strings.Join(strings.Fields(file.FlatNodeText(id)), "")}
		if !strings.HasSuffix(imp.fqn, ".R") {
			continue
		}
		if alias, ok := file.FlatFindChild(header, "import_alias"); ok {
			if name, ok := file.FlatFindChild(alias, "type_identifier"); ok {
				imp.alias = file.FlatNodeText(name)
			}
		}
		out = append(out, imp)
	}
	return out
}

// checkMissingAlias reports an R class import without an alias from another module. Lint calls an import local when
// its package is the project's package; krit, which does not read the module's namespace here, uses Lint's fallback
// for an unknown project package: the import is local when the file's package starts with it. As in Lint, of the
// imports with a configured alias only the file's last one is reported (where Lint attaches its one quick fix); the
// others are reported each.
func (r *ResourceImportAliasRule) checkMissingAlias(ctx *api.Context) {
	file := ctx.File
	pkg := moshiFilePackage(file)
	aliases := r.aliases()
	var root uint32
	for _, imp := range resourceRImports(file, ctx.Idx) {
		if imp.alias != "" {
			continue
		}
		if parent := strings.TrimSuffix(imp.fqn, ".R"); strings.HasPrefix(pkg, parent) {
			continue
		}
		if _, ok := aliases[imp.fqn]; ok {
			root = imp.node
			continue
		}
		ctx.EmitAtNode(imp.node, "Use an import alias for R classes from other modules")
	}
	if root != 0 {
		ctx.EmitAtNode(root, "Use an import alias for R classes from other modules")
	}
}

// checkWrongAlias reports the file's first R class import whose alias differs from the configured one.
func (r *ResourceImportAliasRule) checkWrongAlias(ctx *api.Context) {
	aliases := r.aliases()
	for _, imp := range resourceRImports(ctx.File, ctx.Idx) {
		if want, ok := aliases[imp.fqn]; ok && imp.alias != "" && imp.alias != want {
			ctx.EmitAtNode(imp.node, "Use "+want+" as an import alias here")
			return
		}
	}
}
