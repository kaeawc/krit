package rules

import (
	"github.com/kaeawc/krit/internal/filefacts"
	"github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

const javaIoFile = "java.io.File"

// fileReturningMembers are Android members that return a java.io.File, so a chain ending in one is a File receiver.
var fileReturningMembers = map[string]bool{
	"filesDir": true, "getFilesDir": true, "cacheDir": true, "getCacheDir": true, "codeCacheDir": true,
	"getCodeCacheDir": true, "noBackupFilesDir": true, "getNoBackupFilesDir": true, "obbDir": true, "getObbDir": true,
	"getExternalFilesDir": true, "getExternalCacheDir": true, "getDir": true, "getDataDirectory": true,
	"getExternalStorageDirectory": true, "getDownloadCacheDirectory": true, "getRootDirectory": true,
	"getDatabasePath": true, "getFileStreamPath": true,
}

const slotLintCallsAllocatable = "lintCallsAllocatableBytes"

// UsableSpaceRule mirrors Lint's UsableSpace: File.getUsableSpace() in a file that never asks the
// StorageManager for allocatable bytes.
type UsableSpaceRule struct {
	FlatDispatchBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence: the receiver type is inferred from declarations.
func (r *UsableSpaceRule) Confidence() float64 { return api.ConfidenceMedium }

// lintFileCallsNamed reports whether any call in the file has the given method name.
func lintFileCallsNamed(file *scanner.File, name string) bool {
	return filefacts.FileFact(fileFactsCache(), file, slotLintCallsAllocatable+name, func() bool {
		found := false
		for _, kind := range []string{"call_expression", "method_invocation"} {
			file.FlatWalkNodes(0, kind, func(idx uint32) {
				if call, ok := lintCallAt(file, idx); ok && call.Name == name {
					found = true
				}
			})
		}
		return found
	})
}

func (r *UsableSpaceRule) check(ctx *api.Context) {
	file := ctx.File
	var call lintCall
	var nameNode uint32
	switch file.FlatType(ctx.Idx) {
	case "navigation_expression":
		// A call's callee is reported through the call itself; this is a synthetic property read.
		if parent, ok := file.FlatParent(ctx.Idx); ok && file.FlatType(parent) == "call_expression" {
			if nav, _ := flatCallExpressionParts(file, parent); nav == ctx.Idx {
				return
			}
		}
		if flatNavigationExpressionLastIdentifier(file, ctx.Idx) != "usableSpace" {
			return
		}
		call = lintCall{File: file, Idx: ctx.Idx, Name: "usableSpace"}
		for child := file.FlatFirstChild(ctx.Idx); child != 0; child = file.FlatNextSib(child) {
			if file.FlatType(child) == "navigation_suffix" {
				nameNode, _ = file.FlatFindChild(child, "simple_identifier")
				break
			}
			if file.FlatIsNamed(child) && call.ReceiverNode == 0 {
				call.ReceiverNode = child
			}
		}
		call.ReceiverText = file.FlatNodeText(call.ReceiverNode)
	default:
		var ok bool
		call, ok = lintCallAt(file, ctx.Idx)
		if !ok || call.Name != "getUsableSpace" {
			return
		}
		nameNode = lintCallNameNode(file, ctx.Idx)
	}
	if nameNode == 0 || call.ReceiverNode == 0 {
		return
	}
	if !usableSpaceReceiverIsFile(ctx, call) || lintFileCallsNamed(file, "getAllocatableBytes") {
		return
	}
	ctx.Emit(lintFindingAtNode(ctx, nameNode, "",
		"Consider also using `StorageManager#getAllocatableBytes` and `allocateBytes` which will consider clearable cached data"))
}

func usableSpaceReceiverIsFile(ctx *api.Context, call lintCall) bool {
	file := ctx.File
	recv := call.ReceiverNode
	switch file.FlatType(recv) {
	case "navigation_expression":
		if fileReturningMembers[flatNavigationExpressionLastIdentifier(file, recv)] {
			return true
		}
	case "call_expression", "method_invocation":
		if inner, ok := lintCallAt(file, recv); ok && fileReturningMembers[inner.Name] {
			return true
		}
	}
	return lintReceiverIsType(ctx, call, javaIoFile)
}

// lintCallNameNode returns the identifier node holding the called method's name.
func lintCallNameNode(file *scanner.File, idx uint32) uint32 {
	switch file.FlatType(idx) {
	case "call_expression":
		for child := file.FlatFirstChild(idx); child != 0; child = file.FlatNextSib(child) {
			switch file.FlatType(child) {
			case "simple_identifier":
				return child
			case "navigation_expression":
				var name uint32
				for part := file.FlatFirstChild(child); part != 0; part = file.FlatNextSib(part) {
					if file.FlatType(part) == "navigation_suffix" {
						name, _ = file.FlatFindChild(part, "simple_identifier")
					}
				}
				return name
			}
		}
	case "method_invocation":
		var name uint32
		for child := file.FlatFirstChild(idx); child != 0 && file.FlatType(child) != "argument_list"; child = file.FlatNextSib(child) {
			if file.FlatType(child) == "identifier" {
				name = child
			}
		}
		return name
	}
	return 0
}
