package typeinfer

import (
	"github.com/kaeawc/krit/internal/scanner"
)

// flatFirstOfType walks the flat tree and returns the index of the first node
// whose type matches. Returns 0 if not found.
func flatFirstOfType(file *scanner.File, nodeType string) uint32 {
	if file == nil || file.FlatTree == nil {
		return 0
	}
	var found uint32
	file.FlatWalkAllNodes(0, func(idx uint32) {
		if found == 0 && file.FlatType(idx) == nodeType {
			found = idx
		}
	})
	return found
}

// flatFirstOfTypeWithText walks the flat tree and returns the index of the
// first node of the given type whose text matches. Returns 0 if not found.
func flatFirstOfTypeWithText(file *scanner.File, nodeType, text string) uint32 {
	if file == nil || file.FlatTree == nil {
		return 0
	}
	var found uint32
	file.FlatWalkAllNodes(0, func(idx uint32) {
		if found == 0 && file.FlatType(idx) == nodeType && file.FlatNodeText(idx) == text {
			found = idx
		}
	})
	return found
}
