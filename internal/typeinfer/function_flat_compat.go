package typeinfer

import "github.com/kaeawc/krit/internal/scanner"

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
	if receiverType != "" && foundDot {
		if retType != nil && retType.Kind != TypeUnknown {
			r.extensions = append(r.extensions, &ExtensionFuncInfo{
				ReceiverType: receiverType,
				Name:         funcName,
				ReturnType:   retType,
			})
			r.functions[funcName] = retType
			if pkg != "" {
				r.functions[pkg+"."+funcName] = retType
			}
		}
		return
	}

	if retType != nil && retType.Kind != TypeUnknown {
		r.functions[funcName] = retType
		if pkg != "" {
			r.functions[pkg+"."+funcName] = retType
		}
	}
}
