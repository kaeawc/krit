package rules

import (
	"regexp"
	"strings"

	"github.com/kaeawc/krit/internal/scanner"
)

const preferenceActivityFQN = "android.preference.PreferenceActivity"

var (
	sourcePackageRe = regexp.MustCompile(`(?m)^\s*package\s+([\w.]+)`)
	sourceImportRe  = regexp.MustCompile(`(?m)^\s*import\s+(?:static\s+)?([\w.]+?)(\.\*)?(?:\s+as\s+(\w+))?\s*;?\s*$`)
)

// preferenceClassDecl is one source class with its direct supertype names as written.
type preferenceClassDecl struct {
	fqn    string
	supers []string
	file   *preferenceFileScope
}

// preferenceFileScope holds the name-resolution context of one source file.
type preferenceFileScope struct {
	pkg       string
	imports   map[string]string // simple or alias name -> FQN
	wildcards []string          // packages imported with `.*`
}

// ComputeActivityPreferenceFacts maps each source-declared class FQN to whether its
// superclass chain contains android.preference.PreferenceActivity.
func ComputeActivityPreferenceFacts(files []*scanner.File) map[string]bool {
	var decls []*preferenceClassDecl
	byFQN := map[string]*preferenceClassDecl{}
	for _, file := range files {
		if file == nil {
			continue
		}
		scope := newPreferenceFileScope(string(file.Content))
		file.FlatWalkNodes(0, "class_declaration", func(idx uint32) {
			name := extractIdentifierFlat(file, idx)
			if name == "" {
				return
			}
			d := &preferenceClassDecl{fqn: preferenceClassFQN(file, idx, name, scope.pkg), supers: preferenceDirectSupers(file, idx), file: scope}
			decls = append(decls, d)
			byFQN[d.fqn] = d
		})
	}
	facts := make(map[string]bool, len(decls))
	for _, d := range decls {
		facts[d.fqn] = preferenceChainReaches(d, byFQN, map[string]bool{})
	}
	return facts
}

func newPreferenceFileScope(content string) *preferenceFileScope {
	scope := &preferenceFileScope{imports: map[string]string{}}
	if m := sourcePackageRe.FindStringSubmatch(content); m != nil {
		scope.pkg = m[1]
	}
	for _, m := range sourceImportRe.FindAllStringSubmatch(content, -1) {
		switch {
		case m[2] != "":
			scope.wildcards = append(scope.wildcards, m[1])
		case m[3] != "":
			scope.imports[m[3]] = m[1]
		default:
			scope.imports[m[1][strings.LastIndex(m[1], ".")+1:]] = m[1]
		}
	}
	return scope
}

func preferenceClassFQN(file *scanner.File, idx uint32, name, pkg string) string {
	parts := []string{name}
	for parent, ok := file.FlatParent(idx); ok; parent, ok = file.FlatParent(parent) {
		if file.FlatType(parent) == "class_declaration" {
			if outer := extractIdentifierFlat(file, parent); outer != "" {
				parts = append([]string{outer}, parts...)
			}
		}
	}
	if pkg != "" {
		parts = append([]string{pkg}, parts...)
	}
	return strings.Join(parts, ".")
}

// preferenceDirectSupers returns the superclass and interface names written in the class header.
func preferenceDirectSupers(file *scanner.File, idx uint32) []string {
	var supers []string
	for _, ref := range androidDirectSupertypesFlat(file, idx) {
		supers = append(supers, ref.name)
	}
	for child := file.FlatFirstChild(idx); child != 0; child = file.FlatNextSib(child) {
		if file.FlatType(child) != "superclass" {
			continue
		}
		text := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(file.FlatNodeText(child)), "extends"))
		if cut := strings.Index(text, "<"); cut >= 0 {
			text = text[:cut]
		}
		if text = strings.ReplaceAll(strings.TrimSpace(text), " ", ""); text != "" {
			supers = append(supers, text)
		}
	}
	return supers
}

func preferenceChainReaches(d *preferenceClassDecl, byFQN map[string]*preferenceClassDecl, seen map[string]bool) bool {
	if seen[d.fqn] {
		return false
	}
	seen[d.fqn] = true
	for _, super := range d.supers {
		fqn := resolvePreferenceSuper(d, super, byFQN)
		if fqn == preferenceActivityFQN {
			return true
		}
		if next := byFQN[fqn]; next != nil && preferenceChainReaches(next, byFQN, seen) {
			return true
		}
	}
	return false
}

// resolvePreferenceSuper turns a written supertype name into an FQN using imports,
// the declaring package, and source-declared classes.
func resolvePreferenceSuper(d *preferenceClassDecl, name string, byFQN map[string]*preferenceClassDecl) string {
	if strings.Contains(name, ".") {
		first := name[:strings.Index(name, ".")]
		if imported, ok := d.file.imports[first]; ok {
			return imported + name[len(first):]
		}
		return name
	}
	if imported, ok := d.file.imports[name]; ok {
		return imported
	}
	if outer := d.fqn[:strings.LastIndex(d.fqn, ".")+1]; byFQN[outer+name] != nil {
		return outer + name
	}
	if byFQN[d.file.pkg+"."+name] != nil {
		return d.file.pkg + "." + name
	}
	for _, wildcard := range d.file.wildcards {
		if candidate := wildcard + "." + name; candidate == preferenceActivityFQN || byFQN[candidate] != nil {
			return candidate
		}
	}
	return name
}
