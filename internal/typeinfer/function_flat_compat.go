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
	// ScopeStart and ScopeEnd bound the byte range of the file that can call a
	// FileLocal declaration; ScopeEnd is 0 when the whole file can.
	ScopeStart, ScopeEnd uint32
}

// visibleAt reports whether a call at offset in path can reach a declaration in
// declPath with the given visibility.
func visibleAt(fileLocal bool, scopeStart, scopeEnd uint32, declPath, path string, offset uint32) bool {
	if !fileLocal {
		return true
	}
	if declPath != path {
		return false
	}
	return scopeEnd == 0 || offset >= scopeStart && offset < scopeEnd
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
	decl := FunctionDecl{Type: retType}
	decl.FileLocal, decl.ScopeStart, decl.ScopeEnd = flatDeclarationVisibility(file, funcIdx)
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
			ScopeStart:   decl.ScopeStart,
			ScopeEnd:     decl.ScopeEnd,
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

// summarizeFunctionDecls rebuilds functionDecls and extensions from the merged
// files and precomputes functionSummaries, so the common lookup does not walk
// every same-name declaration.
func (r *defaultResolver) summarizeFunctionDecls() {
	r.extensions = nil
	for _, path := range sortedKeys(r.fileExtensions) {
		r.extensions = append(r.extensions, r.fileExtensions[path]...)
	}
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
		s.public, s.ambiguous = agreedFunctionType(public, "", 0)
		r.functionSummaries[key] = s
	}
}

// lookupFunction returns the return type a call at offset in file sees for
// key, and whether any declaration under key is visible there. Declarations
// private to another file, or local to a body the call is outside of, are
// invisible. Overload resolution is out of reach, so the
// type is nil when the visible candidates disagree or any of them lacks a
// known return type; callers then report an unknown type rather than guess
// from another source. Keys with no recorded declarations fall back to the
// plain name map.
func (r *defaultResolver) lookupFunction(key string, file *scanner.File, offset uint32) (*ResolvedType, bool) {
	decls, ok := r.functionDecls[key]
	if !ok {
		t, found := r.functions[key]
		return t, found
	}
	path := ""
	if file != nil {
		path = file.Path
	}
	if s, ok := r.functionSummaries[key]; ok && !s.localPaths[path] {
		if s.ambiguous {
			return nil, true
		}
		return s.public, s.public != nil
	}
	found, ambiguous := agreedFunctionType(decls, path, offset)
	if ambiguous {
		return nil, true
	}
	return found, found != nil
}

// agreedFunctionType returns the single return type of the declarations
// visible at offset in path, or ambiguous when they disagree or one is unknown.
func agreedFunctionType(decls []functionDeclRef, path string, offset uint32) (found *ResolvedType, ambiguous bool) {
	for _, d := range decls {
		if !visibleAt(d.FileLocal, d.ScopeStart, d.ScopeEnd, d.Path, path, offset) {
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
// visible from file, and whether one is visible, with the same visibility and
// ambiguity rules as lookupFunction.
func (r *defaultResolver) lookupExtension(receiverType, name string, file *scanner.File, offset uint32) (*ResolvedType, bool) {
	path := ""
	if file != nil {
		path = file.Path
	}
	var found *ResolvedType
	for _, ext := range r.extensions {
		if ext.Name != name || ext.ReceiverType != receiverType {
			continue
		}
		if !visibleAt(ext.FileLocal, ext.ScopeStart, ext.ScopeEnd, ext.Path, path, offset) {
			continue
		}
		if ext.ReturnType == nil || ext.ReturnType.Kind == TypeUnknown {
			return nil, true
		}
		if found == nil {
			found = ext.ReturnType
			continue
		}
		if !reflect.DeepEqual(*found, *ext.ReturnType) {
			return nil, true
		}
	}
	return found, found != nil
}

// knownOrUnknown returns t, or an unknown type when a visible declaration
// exists but its type could not be settled.
func knownOrUnknown(t *ResolvedType) *ResolvedType {
	if t == nil {
		return UnknownType()
	}
	return t
}

// flatDeclarationVisibility reports whether only its own file can reference the
// declaration at idx, and the byte range of the file that can. A function in a
// body, lambda or object literal is limited to that node; a private member to
// its outermost class; a private top-level function or a member of a private
// class to the whole file (end 0).
func flatDeclarationVisibility(file *scanner.File, idx uint32) (fileLocal bool, start, end uint32) {
	if file == nil || idx == 0 {
		return false, 0, 0
	}
	private := flatReadModifierFlags(file, idx).visibility == "private"
	privateOwner := false
	var outerClass uint32
	for p, ok := file.FlatParent(idx); ok && p != 0; p, ok = file.FlatParent(p) {
		switch file.FlatType(p) {
		case "function_body", "lambda_literal", "anonymous_initializer", "object_literal":
			return true, file.FlatStartByte(p), file.FlatEndByte(p)
		case "class_declaration", "object_declaration", "companion_object":
			if flatReadModifierFlags(file, p).visibility == "private" {
				privateOwner = true
			}
			if file.FlatType(p) != "companion_object" {
				outerClass = p
			}
		}
	}
	switch {
	case private && outerClass != 0:
		return true, file.FlatStartByte(outerClass), file.FlatEndByte(outerClass)
	case private || privateOwner:
		return true, 0, 0
	}
	return false, 0, 0
}
