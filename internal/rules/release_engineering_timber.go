package rules

import (
	"strings"

	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/rules/semantics"
	"github.com/kaeawc/krit/internal/scanner"
	"github.com/kaeawc/krit/internal/sourceheader"
	"github.com/kaeawc/krit/internal/typeinfer"
)

// TimberTreeNotPlantedRule flags projects that use Timber.d/i/w/e but have
// no Timber.plant() call reachable from Application.onCreate.
type TimberTreeNotPlantedRule struct {
	BaseRule
}

func (r *TimberTreeNotPlantedRule) Confidence() float64 { return api.ConfidenceMedium }
func (r *TimberTreeNotPlantedRule) check(ctx *api.Context) {
	files := timberProjectFiles(ctx)
	if len(files) == 0 {
		return
	}

	var firstUsage *timberCallSite
	hasStartupPlant := false

	for _, file := range files {
		if file == nil || file.FlatTree == nil || scanner.IsTestFile(file.Path) {
			continue
		}
		imports := timberImportsForFile(file)
		file.FlatWalkNodes(0, "call_expression", func(call uint32) {
			site, ok := resolveTimberCall(ctx, file, call, imports)
			if !ok {
				return
			}
			if site.kind == timberCallPlant {
				if timberPlantInApplicationOnCreate(ctx, file, call) {
					hasStartupPlant = true
				}
				return
			}
			if firstUsage == nil || site.before(*firstUsage) {
				firstUsage = &site
			}
		})
	}

	if firstUsage == nil || hasStartupPlant {
		return
	}

	ctx.Emit(scanner.Finding{
		File:       firstUsage.file.Path,
		Line:       firstUsage.line,
		Col:        firstUsage.col,
		RuleSet:    r.RuleSetName,
		Rule:       r.RuleName,
		Severity:   r.Sev,
		Message:    "Timber is used but Timber.plant() is not called from Application.onCreate; logs will be silently dropped.",
		Confidence: firstUsage.confidence,
	})
}

type timberCallKind int

const (
	timberCallUsage timberCallKind = iota
	timberCallPlant
)

type timberCallSite struct {
	kind       timberCallKind
	file       *scanner.File
	call       uint32
	line       int
	col        int
	confidence float64
}

func (s timberCallSite) before(other timberCallSite) bool {
	if s.file.Path != other.file.Path {
		return s.file.Path < other.file.Path
	}
	if s.line != other.line {
		return s.line < other.line
	}
	return s.col < other.col
}

var timberLoggingCallees = map[string]bool{
	"v":   true,
	"d":   true,
	"i":   true,
	"w":   true,
	"e":   true,
	"wtf": true,
}

type timberImportInfo struct {
	receivers map[string]bool
	members   map[string]bool
	wildcard  bool
}

func timberProjectFiles(ctx *api.Context) []*scanner.File {
	if ctx.CodeIndex != nil && len(ctx.CodeIndex.Files) > 0 {
		return ctx.CodeIndex.Files
	}
	return ctx.ParsedFiles
}

func timberImportsForFile(file *scanner.File) timberImportInfo {
	info := timberImportInfo{
		receivers: map[string]bool{"timber.log.Timber": true},
		members:   make(map[string]bool),
	}
	if file == nil || file.FlatTree == nil {
		return info
	}
	file.FlatWalkNodes(0, "import_header", func(node uint32) {
		text := sourceheader.FirstHeaderLine(file.FlatNodeText(node), "import")
		if text == "" {
			return
		}
		path := text
		alias := ""
		if idx := strings.Index(path, " as "); idx >= 0 {
			alias = strings.TrimSpace(path[idx+4:])
			path = strings.TrimSpace(path[:idx])
		}
		switch {
		case path == "timber.log.Timber":
			name := "Timber"
			if alias != "" {
				name = alias
			}
			info.receivers[name] = true
		case strings.HasPrefix(path, "timber.log.Timber."):
			member := strings.TrimPrefix(path, "timber.log.Timber.")
			if member == "*" {
				info.wildcard = true
				return
			}
			if alias != "" {
				member = alias
			}
			info.members[member] = true
		}
	})
	return info
}

func resolveTimberCall(parent *api.Context, file *scanner.File, call uint32, imports timberImportInfo) (timberCallSite, bool) {
	localCtx := &api.Context{File: file, Resolver: parent.Resolver, CodeIndex: parent.CodeIndex}
	target, ok := semantics.ResolveCallTarget(localCtx, call)
	if !ok {
		return timberCallSite{}, false
	}

	if target.Resolved {
		if kind, ok := resolvedTimberCallKind(target.QualifiedName); ok {
			return newTimberCallSite(file, call, kind, 0.9), true
		}
		return timberCallSite{}, false
	}

	kind, calleeOK := timberCallKindForName(target.CalleeName)
	if !calleeOK {
		return timberCallSite{}, false
	}

	if target.Receiver.Valid() {
		receiver := timberQualifiedPath(file, target.Receiver.Node)
		if timberReceiverMatches(receiver, imports) {
			return newTimberCallSite(file, call, kind, 0.7), true
		}
		return timberCallSite{}, false
	}

	if imports.wildcard || imports.members[target.CalleeName] {
		return newTimberCallSite(file, call, kind, 0.7), true
	}
	return timberCallSite{}, false
}

func newTimberCallSite(file *scanner.File, call uint32, kind timberCallKind, confidence float64) timberCallSite {
	return timberCallSite{
		kind:       kind,
		file:       file,
		call:       call,
		line:       file.FlatRow(call) + 1,
		col:        file.FlatCol(call) + 1,
		confidence: confidence,
	}
}

func timberCallKindForName(name string) (timberCallKind, bool) {
	if name == "plant" {
		return timberCallPlant, true
	}
	if timberLoggingCallees[name] {
		return timberCallUsage, true
	}
	return timberCallUsage, false
}

func resolvedTimberCallKind(qualifiedName string) (timberCallKind, bool) {
	name := strings.TrimSpace(qualifiedName)
	if paren := strings.Index(name, "("); paren >= 0 {
		name = name[:paren]
	}
	name = strings.ReplaceAll(name, "#", ".")
	if !strings.HasPrefix(name, "timber.log.Timber.") {
		return timberCallUsage, false
	}
	return timberCallKindForName(timberSimpleName(name))
}

func timberReceiverMatches(receiver string, imports timberImportInfo) bool {
	if receiver == "" {
		return false
	}
	if receiver == "timber.log.Timber" || imports.receivers[receiver] {
		return true
	}
	return false
}

func timberPlantInApplicationOnCreate(ctx *api.Context, file *scanner.File, call uint32) bool {
	fn, ok := timberEnclosingAncestor(file, call, "function_declaration")
	if !ok || semantics.DeclarationName(file, fn) != "onCreate" || !file.FlatHasModifier(fn, "override") {
		return false
	}
	cls, ok := timberEnclosingAncestor(file, fn, "class_declaration", "object_declaration")
	if !ok {
		return false
	}
	return timberClassExtendsApplication(file, cls, ctx.Resolver, make(map[uint32]bool))
}

func timberClassExtendsApplication(file *scanner.File, classNode uint32, resolver typeinfer.TypeResolver, seen map[uint32]bool) bool {
	if classNode == 0 || seen[classNode] {
		return false
	}
	seen[classNode] = true

	localDecls := androidSameFileClassDeclarations(file)
	for _, super := range androidDirectSupertypesFlat(file, classNode) {
		if super.simple == "" {
			continue
		}
		if timberSupertypeIsLocalApplication(file, super, classNode, resolver, localDecls, seen) {
			return true
		}
		if timberSupertypeIsAndroidApplication(file, super, resolver) {
			return true
		}
	}
	return false
}

func timberSupertypeIsLocalApplication(file *scanner.File, super androidSupertypeRef, classNode uint32, resolver typeinfer.TypeResolver, localDecls map[string][]androidClassDecl, seen map[uint32]bool) bool {
	for _, decl := range localDecls[super.simple] {
		if decl.idx == classNode {
			continue
		}
		if timberClassExtendsApplication(file, decl.idx, resolver, seen) {
			return true
		}
		if super.simple == "Application" {
			return false
		}
	}
	return false
}

func timberSupertypeIsAndroidApplication(file *scanner.File, super androidSupertypeRef, resolver typeinfer.TypeResolver) bool {
	if super.name == "android.app.Application" {
		return true
	}
	if resolver != nil {
		name := super.name
		if !super.qualified {
			if fqn := resolver.ResolveImport(super.simple, file); fqn != "" {
				name = fqn
			}
		}
		if name == "android.app.Application" {
			return true
		}
		if info := resolver.ClassHierarchy(name); info != nil {
			for _, parent := range info.Supertypes {
				if parent == "android.app.Application" {
					return true
				}
			}
		}
	}
	return super.simple == "Application" && !super.qualified
}

func timberQualifiedPath(file *scanner.File, idx uint32) string {
	if file == nil || idx == 0 {
		return ""
	}
	switch file.FlatType(idx) {
	case "simple_identifier", "type_identifier":
		return file.FlatNodeText(idx)
	case "navigation_expression":
		var parts []string
		file.FlatWalkAllNodes(idx, func(node uint32) {
			switch file.FlatType(node) {
			case "simple_identifier", "type_identifier":
				parts = append(parts, file.FlatNodeText(node))
			}
		})
		return strings.Join(parts, ".")
	case "call_expression":
		target, ok := semantics.ResolveCallTarget(&api.Context{File: file}, idx)
		if ok && target.Receiver.Valid() {
			return timberQualifiedPath(file, target.Receiver.Node)
		}
	}
	return ""
}

func timberEnclosingAncestor(file *scanner.File, idx uint32, types ...string) (uint32, bool) {
	for p, ok := file.FlatParent(idx); ok; p, ok = file.FlatParent(p) {
		t := file.FlatType(p)
		for _, want := range types {
			if t == want {
				return p, true
			}
		}
	}
	return 0, false
}
