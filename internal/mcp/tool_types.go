package mcp

import (
	"encoding/json"

	"github.com/kaeawc/krit/internal/typeinfer"
)

// typesArgs are the arguments for the types tool.
type typesArgs struct {
	Query string `json:"query"`
	Code  string `json:"code"`
	Path  string `json:"path"`
}

// toolTypes parses Kotlin code, runs type inference, and returns the
// requested type information.
func (s *Server) toolTypes(arguments json.RawMessage) ToolResult {
	var args typesArgs
	if err := json.Unmarshal(arguments, &args); err != nil {
		return errorResult("invalid arguments: " + err.Error())
	}
	if args.Code == "" {
		return errorResult("'code' argument is required")
	}
	if args.Query == "" {
		return errorResult("'query' argument is required")
	}

	path := args.Path
	if path == "" {
		path = "input.kt"
	}

	file, err := parseKotlinCode(args.Code, path)
	if err != nil {
		return errorResult(err.Error())
	}

	info := typeinfer.IndexFileParallel(file)
	if info == nil {
		return errorResult("type inference returned no results")
	}

	switch args.Query {
	case queryTypesClasses:
		return typesQueryClasses(info)
	case queryTypesHierarchy:
		return typesQueryHierarchy(info)
	case queryTypesImports:
		return typesQueryImports(info)
	case queryTypesFunctionSigs:
		return typesQueryFunctions(info)
	case queryTypesSealedVariants:
		return typesQuerySealed(info)
	case queryTypesEnumEntries:
		return typesQueryEnum(info)
	default:
		return errorResult("unknown query type: " + args.Query + "; valid: " + formatList(typesQueries))
	}
}

func typesQueryClasses(info *typeinfer.FileTypeInfo) ToolResult {
	type classJSON struct {
		Name       string   `json:"name"`
		FQN        string   `json:"fqn,omitempty"`
		Kind       string   `json:"kind"`
		Supertypes []string `json:"supertypes,omitempty"`
		IsSealed   bool     `json:"isSealed,omitempty"`
		IsData     bool     `json:"isData,omitempty"`
		IsAbstract bool     `json:"isAbstract,omitempty"`
		IsOpen     bool     `json:"isOpen,omitempty"`
	}
	classes := make([]classJSON, 0, len(info.Classes))
	for _, ci := range info.Classes {
		classes = append(classes, classJSON{
			Name:       ci.Name,
			FQN:        ci.FQN,
			Kind:       ci.Kind,
			Supertypes: ci.Supertypes,
			IsSealed:   ci.IsSealed,
			IsData:     ci.IsData,
			IsAbstract: ci.IsAbstract,
			IsOpen:     ci.IsOpen,
		})
	}
	return jsonResult(classes)
}

func typesQueryHierarchy(info *typeinfer.FileTypeInfo) ToolResult {
	type hierarchyEntry struct {
		Name       string   `json:"name"`
		Supertypes []string `json:"supertypes"`
	}
	entries := make([]hierarchyEntry, 0, len(info.Classes))
	for _, ci := range info.Classes {
		entries = append(entries, hierarchyEntry{
			Name:       ci.Name,
			Supertypes: ci.Supertypes,
		})
	}
	return jsonResult(entries)
}

func typesQueryImports(info *typeinfer.FileTypeInfo) ToolResult {
	type importsJSON struct {
		Explicit map[string]string `json:"explicit"`
		Wildcard []string          `json:"wildcard"`
		Aliases  map[string]string `json:"aliases,omitempty"`
	}
	imp := importsJSON{
		Explicit: make(map[string]string),
		Wildcard: []string{},
	}
	if info.ImportTable != nil {
		if info.ImportTable.Explicit != nil {
			imp.Explicit = info.ImportTable.Explicit
		}
		if info.ImportTable.Wildcard != nil {
			imp.Wildcard = info.ImportTable.Wildcard
		}
		if len(info.ImportTable.Aliases) > 0 {
			imp.Aliases = info.ImportTable.Aliases
		}
	}
	return jsonResult(imp)
}

func typesQueryFunctions(info *typeinfer.FileTypeInfo) ToolResult {
	type funcJSON struct {
		Name       string `json:"name"`
		ReturnType string `json:"returnType,omitempty"`
	}
	funcs := make([]funcJSON, 0, len(info.Functions))
	for name, rt := range info.Functions {
		retType := ""
		if rt != nil {
			retType = rt.Name
		}
		funcs = append(funcs, funcJSON{
			Name:       name,
			ReturnType: retType,
		})
	}
	return jsonResult(funcs)
}

func typesQuerySealed(info *typeinfer.FileTypeInfo) ToolResult {
	if info.SealedSubs == nil {
		return jsonResult(map[string][]string{})
	}
	return jsonResult(info.SealedSubs)
}

func typesQueryEnum(info *typeinfer.FileTypeInfo) ToolResult {
	if info.EnumEntries == nil {
		return jsonResult(map[string][]string{})
	}
	return jsonResult(info.EnumEntries)
}
