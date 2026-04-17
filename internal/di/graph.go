package di

import (
	"sort"
	"strings"

	"github.com/kaeawc/krit/internal/module"
	"github.com/kaeawc/krit/internal/scanner"
)

var dependencyWrappers = map[string]bool{
	"Lazy":                  true,
	"dagger.Lazy":           true,
	"Provider":              true,
	"javax.inject.Provider": true,
}

// Graph is a lightweight DI binding graph built from constructor-injected
// classes. It is intentionally small but reusable by future whole-graph rules.
type Graph struct {
	Bindings    map[string]*Binding
	simpleNames map[string][]string
}

// Binding describes a constructor-injected type in the source graph.
type Binding struct {
	Name         string
	FQN          string
	PackageName  string
	ModulePath   string
	File         string
	Line         int
	Scope        Scope
	Dependencies []Dependency
}

// Dependency describes a constructor parameter edge in the binding graph.
type Dependency struct {
	ParameterName string
	TypeName      string
	Target        string
}

// ScopeViolation describes a wider-scoped root that can reach a narrower-scoped
// dependency through constructor-injection edges.
type ScopeViolation struct {
	Root     *Binding
	Offender *Binding
	Path     []string
}

type pendingDependency struct {
	parameterName string
	typeName      string
}

type bindingSeed struct {
	binding      *Binding
	packageName  string
	imports      map[string]string
	dependencies []pendingDependency
}

// BuildGraph indexes constructor-injected classes and resolves their direct
// dependencies to other indexed bindings. When moduleGraph is non-nil, each
// binding is tagged with the Gradle module that owns its source file.
func BuildGraph(files []*scanner.File, moduleGraph *module.ModuleGraph) *Graph {
	graph := &Graph{
		Bindings:    make(map[string]*Binding),
		simpleNames: make(map[string][]string),
	}

	var seeds []bindingSeed
	for _, file := range files {
		if file == nil || file.FlatTree == nil || len(file.FlatTree.Nodes) == 0 {
			continue
		}
		pkg := packageNameFlat(file)
		imports := importTableFlat(file)
		modulePath := ""
		if moduleGraph != nil {
			modulePath = moduleGraph.FileToModule(file.Path)
		}

		file.FlatWalkNodes(0, "class_declaration", func(idx uint32) {
			seed, ok := buildBindingSeedFlat(idx, file, pkg, imports, modulePath)
			if !ok {
				return
			}
			graph.Bindings[seed.binding.FQN] = seed.binding
			graph.simpleNames[seed.binding.Name] = append(graph.simpleNames[seed.binding.Name], seed.binding.FQN)
			seeds = append(seeds, seed)
		})
	}

	for name := range graph.simpleNames {
		sort.Strings(graph.simpleNames[name])
	}

	for _, seed := range seeds {
		for _, dep := range seed.dependencies {
			target := graph.resolveType(dep.typeName, seed.packageName, seed.imports)
			seed.binding.Dependencies = append(seed.binding.Dependencies, Dependency{
				ParameterName: dep.parameterName,
				TypeName:      dep.typeName,
				Target:        target,
			})
		}
	}

	return graph
}

func buildBindingSeedFlat(idx uint32, file *scanner.File, pkg string, imports map[string]string, modulePath string) (bindingSeed, bool) {
	ctor := file.FlatFindChild(idx, "primary_constructor")
	if ctor == 0 || !hasAnnotationNamedFlat(file, ctor, "Inject") {
		return bindingSeed{}, false
	}

	name := declarationNameFlat(file, idx)
	if name == "" {
		return bindingSeed{}, false
	}

	fqn := name
	if pkg != "" {
		fqn = pkg + "." + name
	}

	seed := bindingSeed{
		binding: &Binding{
			Name:        name,
			FQN:         fqn,
			PackageName: pkg,
			ModulePath:  modulePath,
			File:        file.Path,
			Line:        file.FlatRow(idx) + 1,
			Scope:       detectScopeFlat(file, idx),
		},
		packageName: pkg,
		imports:     imports,
	}

	file.FlatWalkNodes(ctor, "class_parameter", func(param uint32) {
		typeNode := firstTypeNodeFlat(file, param)
		typeName := ""
		if typeNode != 0 {
			typeName = normalizeTypeRef(file.FlatNodeText(typeNode))
		}
		if typeName == "" {
			return
		}
		seed.dependencies = append(seed.dependencies, pendingDependency{
			parameterName: parameterNameFlat(file, param),
			typeName:      typeName,
		})
	})

	return seed, true
}

func detectScopeFlat(file *scanner.File, idx uint32) Scope {
	for _, ann := range annotationNamesFlat(file, idx) {
		scope := ResolveScope(ann)
		if scope.IsSet() {
			return scope
		}
	}
	return Scope{}
}

func hasAnnotationNamedFlat(file *scanner.File, idx uint32, want string) bool {
	for _, ann := range annotationNamesFlat(file, idx) {
		if simpleName(ann) == want {
			return true
		}
	}
	return false
}

func annotationNamesFlat(file *scanner.File, idx uint32) []string {
	var names []string
	addFrom := func(parent uint32) {
		if parent == 0 {
			return
		}
		file.FlatWalkNodes(parent, "annotation", func(child uint32) {
			if name := annotationName(file.FlatNodeText(child)); name != "" {
				names = append(names, name)
			}
		})
	}
	addFrom(file.FlatFindChild(idx, "modifiers"))
	for i := 0; i < file.FlatChildCount(idx); i++ {
		child := file.FlatChild(idx, i)
		if child != 0 && file.FlatType(child) == "annotation" {
			if name := annotationName(file.FlatNodeText(child)); name != "" {
				names = append(names, name)
			}
		}
	}
	return names
}

func packageNameFlat(file *scanner.File) string {
	header := file.FlatFindChild(0, "package_header")
	if header == 0 {
		return ""
	}
	text := strings.TrimSpace(file.FlatNodeText(header))
	text = strings.TrimPrefix(text, "package")
	return strings.TrimSpace(text)
}

func importTableFlat(file *scanner.File) map[string]string {
	imports := make(map[string]string)
	for i := 0; i < file.FlatChildCount(0); i++ {
		node := file.FlatChild(0, i)
		if node == 0 || file.FlatType(node) != "import_header" {
			continue
		}
		text := strings.TrimSpace(file.FlatNodeText(node))
		text = strings.TrimPrefix(text, "import")
		text = strings.TrimSpace(text)
		if text == "" {
			continue
		}

		target := text
		alias := ""
		if before, after, ok := strings.Cut(text, " as "); ok {
			target = strings.TrimSpace(before)
			alias = strings.TrimSpace(after)
		}
		if strings.HasSuffix(target, ".*") {
			continue
		}
		key := alias
		if key == "" {
			key = simpleName(target)
		}
		if key != "" {
			imports[key] = target
		}
	}
	return imports
}

func annotationName(text string) string {
	text = strings.TrimSpace(text)
	if !strings.HasPrefix(text, "@") {
		return ""
	}
	text = strings.TrimPrefix(text, "@")
	if idx := strings.Index(text, ":"); idx >= 0 {
		text = text[idx+1:]
	}
	end := len(text)
	for i, r := range text {
		switch {
		case r == '(' || r == '[' || r == ' ' || r == '\n' || r == '\t':
			end = i
			return text[:end]
		}
	}
	return text[:end]
}

func declarationNameFlat(file *scanner.File, idx uint32) string {
	for i := 0; i < file.FlatChildCount(idx); i++ {
		child := file.FlatChild(idx, i)
		switch file.FlatType(child) {
		case "type_identifier", "simple_identifier":
			return file.FlatNodeText(child)
		}
	}
	return ""
}

func parameterNameFlat(file *scanner.File, idx uint32) string {
	for i := 0; i < file.FlatChildCount(idx); i++ {
		child := file.FlatChild(idx, i)
		if file.FlatType(child) == "simple_identifier" {
			return file.FlatNodeText(child)
		}
	}
	return ""
}

func firstTypeNodeFlat(file *scanner.File, idx uint32) uint32 {
	for i := 0; i < file.FlatChildCount(idx); i++ {
		child := file.FlatChild(idx, i)
		switch file.FlatType(child) {
		case "user_type", "nullable_type", "type_identifier":
			return child
		}
	}
	return 0
}

// Binding returns the indexed binding for the given fully-qualified name.
func (g *Graph) Binding(fqn string) *Binding {
	if g == nil {
		return nil
	}
	return g.Bindings[fqn]
}

// ScopeViolations returns wider-scoped roots that can transitively reach a
// narrower-scoped binding.
func (g *Graph) ScopeViolations() []ScopeViolation {
	if g == nil {
		return nil
	}

	keys := make([]string, 0, len(g.Bindings))
	for key := range g.Bindings {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	var violations []ScopeViolation
	for _, key := range keys {
		root := g.Bindings[key]
		if root == nil || !root.Scope.Known {
			continue
		}
		seenOffenders := make(map[string]bool)
		type state struct {
			binding *Binding
			path    []string
		}
		stack := []state{{binding: root, path: []string{root.FQN}}}
		visited := map[string]bool{root.FQN: true}
		for len(stack) > 0 {
			last := len(stack) - 1
			current := stack[last]
			stack = stack[:last]
			for _, dep := range current.binding.Dependencies {
				next := g.Binding(dep.Target)
				if next == nil {
					continue
				}
				path := append(append([]string{}, current.path...), next.FQN)
				if root.Scope.WiderThan(next.Scope) && !seenOffenders[next.FQN] {
					violations = append(violations, ScopeViolation{
						Root:     root,
						Offender: next,
						Path:     path,
					})
					seenOffenders[next.FQN] = true
				}
				if visited[next.FQN] {
					continue
				}
				visited[next.FQN] = true
				stack = append(stack, state{binding: next, path: path})
			}
		}
	}

	return violations
}

func (g *Graph) resolveType(typeName, pkg string, imports map[string]string) string {
	typeName = normalizeTypeRef(typeName)
	if typeName == "" {
		return ""
	}
	if _, ok := g.Bindings[typeName]; ok {
		return typeName
	}

	if imported := imports[typeName]; imported != "" {
		if _, ok := g.Bindings[imported]; ok {
			return imported
		}
	}

	if strings.Contains(typeName, ".") {
		return typeName
	}

	if pkg != "" {
		candidate := pkg + "." + typeName
		if _, ok := g.Bindings[candidate]; ok {
			return candidate
		}
	}

	matches := g.simpleNames[typeName]
	if len(matches) == 1 {
		return matches[0]
	}
	return ""
}

func normalizeTypeRef(text string) string {
	text = strings.TrimSpace(text)
	text = strings.TrimSuffix(text, "?")
	text = strings.TrimPrefix(text, "in ")
	text = strings.TrimPrefix(text, "out ")
	if outer, arg, ok := splitOuterAndFirstArg(text); ok && dependencyWrappers[outer] {
		return normalizeTypeRef(arg)
	}
	return stripGenericArgs(text)
}

func splitOuterAndFirstArg(text string) (string, string, bool) {
	start := strings.Index(text, "<")
	if start < 0 {
		return "", "", false
	}
	end := matchingAngleIndex(text, start)
	if end < 0 {
		return "", "", false
	}
	outer := strings.TrimSpace(text[:start])
	inner := text[start+1 : end]
	first := firstTopLevelTypeArg(inner)
	if outer == "" || first == "" {
		return "", "", false
	}
	return outer, first, true
}

func stripGenericArgs(text string) string {
	start := strings.Index(text, "<")
	if start < 0 {
		return strings.TrimSpace(text)
	}
	return strings.TrimSpace(text[:start])
}

func matchingAngleIndex(text string, start int) int {
	depth := 0
	for i := start; i < len(text); i++ {
		switch text[i] {
		case '<':
			depth++
		case '>':
			depth--
			if depth == 0 {
				return i
			}
		}
	}
	return -1
}

func firstTopLevelTypeArg(text string) string {
	depth := 0
	for i := 0; i < len(text); i++ {
		switch text[i] {
		case '<':
			depth++
		case '>':
			depth--
		case ',':
			if depth == 0 {
				return strings.TrimSpace(text[:i])
			}
		}
	}
	return strings.TrimSpace(text)
}
