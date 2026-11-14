package typeinfer

import (
	"reflect"

	"github.com/kaeawc/krit/internal/scanner"
)

// FunctionDecl is one source declaration indexed under a function key.
// Type is nil when the declaration has no explicit return type.
type FunctionDecl struct {
	Type *ResolvedType
	// FileLocal marks a declaration that only its own file can call: a
	// private, local or anonymous-object function, or a member of a private class.
	FileLocal bool
}

// functionDeclRef is a FunctionDecl tagged with the file that declares it.
type functionDeclRef struct {
	Path string
	FunctionDecl
}

// indexFunctionFlat keeps declaration indexing on the flat path while the
// remaining declarations work lands in its owner track.
func (r *defaultResolver) indexFunctionFlat(funcIdx uint32, file *scanner.File, it *ImportTable, pkg string) {
	if file == nil || funcIdx == 0 || it == nil {
		return
	}

	receiverType, funcName, foundDot := flatFunctionSignature(file, funcIdx)
	if funcName == "" {
		funcName = flatMemberName(file, funcIdx)
	}
	if funcName == "" {
		funcName = flatFindIdentifier(file, funcIdx)
	}
	if funcName == "" {
		return
	}

	retType := r.extractFunctionReturnTypeFlat(funcIdx, file, it)
	decl := FunctionDecl{Type: retType, FileLocal: flatDeclarationFileLocal(file, funcIdx)}
	r.addFunctionDecl(file.Path, funcName, decl)
	if pkg != "" {
		r.addFunctionDecl(file.Path, pkg+"."+funcName, decl)
	}
	known := retType != nil && retType.Kind != TypeUnknown
	if receiverType != "" && foundDot {
		r.extensions = append(r.extensions, &ExtensionFuncInfo{
			ReceiverType: receiverType,
			Name:         funcName,
			ReturnType:   retType,
			Path:         file.Path,
			FileLocal:    decl.FileLocal,
		})
	}
	if known {
		r.functions[funcName] = retType
		if pkg != "" {
			r.functions[pkg+"."+funcName] = retType
		}
	}
}

// addFunctionDecl records a declaration candidate for key, declared in path.
func (r *defaultResolver) addFunctionDecl(path, key string, decl FunctionDecl) {
	if r.functionDecls == nil {
		r.functionDecls = make(map[string][]functionDeclRef)
	}
	r.functionDecls[key] = append(r.functionDecls[key], functionDeclRef{Path: path, FunctionDecl: decl})
}

// functionSummary is the precomputed lookup answer for callers in files that
// declare no file-local function under the key.
type functionSummary struct {
	public     *ResolvedType   // the agreed type of the non-file-local declarations
	ambiguous  bool            // they disagree, or one has no known type
	localPaths map[string]bool // files with a file-local declaration under the key
}

// summarizeFunctionDecls rebuilds functionDecls from the merged files and
// precomputes functionSummaries, so the common lookup does not walk every
// same-name declaration.
func (r *defaultResolver) summarizeFunctionDecls() {
	r.functionDecls = make(map[string][]functionDeclRef)
	for _, path := range sortedKeys(r.fileFunctionDecls) {
		decls := r.fileFunctionDecls[path]
		for _, key := range sortedKeys(decls) {
			for _, decl := range decls[key] {
				r.addFunctionDecl(path, key, decl)
			}
		}
	}
	r.functionSummaries = make(map[string]functionSummary, len(r.functionDecls))
	for key, decls := range r.functionDecls {
		var s functionSummary
		var public []functionDeclRef
		for _, d := range decls {
			if d.FileLocal {
				if s.localPaths == nil {
					s.localPaths = make(map[string]bool)
				}
				s.localPaths[d.Path] = true
				continue
			}
			public = append(public, d)
		}
		s.public, s.ambiguous = agreedFunctionType(public, "")
		r.functionSummaries[key] = s
	}
}

// lookupFunction returns the return type the call site in file sees for key.
// Declarations private to another file are invisible. Overload resolution is
// out of reach, so it returns nil when the visible candidates disagree or any
// of them lacks a known return type. Keys with no recorded declarations fall
// back to the plain name map.
func (r *defaultResolver) lookupFunction(key string, file *scanner.File) *ResolvedType {
	decls, ok := r.functionDecls[key]
	if !ok {
		return r.functions[key]
	}
	path := ""
	if file != nil {
		path = file.Path
	}
	if s, ok := r.functionSummaries[key]; ok && !s.localPaths[path] {
		if s.ambiguous {
			return nil
		}
		return s.public
	}
	found, ambiguous := agreedFunctionType(decls, path)
	if ambiguous {
		return nil
	}
	return found
}

// agreedFunctionType returns the single return type of the declarations
// visible from path, or ambiguous when they disagree or one is unknown.
func agreedFunctionType(decls []functionDeclRef, path string) (found *ResolvedType, ambiguous bool) {
	for _, d := range decls {
		if d.FileLocal && d.Path != path {
			continue
		}
		if d.Type == nil || d.Type.Kind == TypeUnknown {
			return nil, true
		}
		if found == nil {
			found = d.Type
			continue
		}
		if !reflect.DeepEqual(*found, *d.Type) {
			return nil, true
		}
	}
	return found, false
}

// lookupExtension returns the return type of the extension receiverType.name
// visible from file, with the same visibility and ambiguity rules as lookupFunction.
func (r *defaultResolver) lookupExtension(receiverType, name string, file *scanner.File) *ResolvedType {
	path := ""
	if file != nil {
		path = file.Path
	}
	var found *ResolvedType
	for _, ext := range r.extensions {
		if ext.Name != name || ext.ReceiverType != receiverType {
			continue
		}
		if ext.FileLocal && ext.Path != path {
			continue
		}
		if ext.ReturnType == nil || ext.ReturnType.Kind == TypeUnknown {
			return nil
		}
		if found == nil {
			found = ext.ReturnType
			continue
		}
		if !reflect.DeepEqual(*found, *ext.ReturnType) {
			return nil
		}
	}
	return found
}

// flatDeclarationFileLocal reports whether code outside the file cannot
// reference the declaration at idx: it is private, declared inside a function
// body, lambda or object literal, or nested in a private class.
func flatDeclarationFileLocal(file *scanner.File, idx uint32) bool {
	if file == nil || idx == 0 {
		return false
	}
	if flatReadModifierFlags(file, idx).visibility == "private" {
		return true
	}
	for p, ok := file.FlatParent(idx); ok && p != 0; p, ok = file.FlatParent(p) {
		switch file.FlatType(p) {
		case "function_body", "lambda_literal", "anonymous_initializer", "object_literal":
			return true
		case "class_declaration", "object_declaration", "companion_object":
			if flatReadModifierFlags(file, p).visibility == "private" {
				return true
			}
		}
	}
	return false
}
