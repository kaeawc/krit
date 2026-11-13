package rules

import (
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// fragmentExtendsKnownBase reports whether a delegation specifier of the declaration names a Fragment base class.
func fragmentExtendsKnownBase(file *scanner.File, decl uint32) bool {
	for i := 0; i < file.FlatChildCount(decl); i++ {
		child := file.FlatChild(decl, i)
		if file.FlatType(child) != "delegation_specifier" {
			continue
		}
		typeName := viewConstructorSupertypeNameFlat(file, child)
		for _, base := range fragmentSuperclasses {
			if typeName == base {
				return true
			}
		}
	}
	return false
}

// fragmentConstructorCheck reports Fragment subclasses the framework cannot re-instantiate: a missing no-arg
// constructor, a private or protected class or no-arg constructor, an inner class, or an anonymous object.
// Kotlin `internal` compiles to a public JVM class and constructor, so unlike Android Lint it is accepted.
func fragmentConstructorCheck(ctx *api.Context) {
	idx, file := ctx.Idx, ctx.File
	if file.FlatHasModifier(idx, "abstract") || file.FlatHasModifier(idx, "sealed") {
		return
	}
	// Test fragments are built by the test, not re-instantiated by the framework.
	if scanner.IsTestFile(file.Path) || !fragmentExtendsKnownBase(file, idx) {
		return
	}
	if file.FlatType(idx) == "object_literal" {
		ctx.EmitAt(declLine(file, idx), declCol(file, idx),
			"Fragments should be static such that they can be re-instantiated by the system, and anonymous classes are not static.")
		return
	}
	name := extractIdentifierFlat(file, idx)
	if fragmentLaunchedFromInstance(file, name) {
		return
	}
	ctorState := fragmentConstructorStateFlat(file, idx)
	// DI-annotated constructors are built by a custom FragmentFactory.
	if ctorState.hasInjectCtor {
		return
	}
	switch {
	case file.FlatHasModifier(idx, "private") || file.FlatHasModifier(idx, "protected"):
		ctx.EmitAt(declLine(file, idx), declCol(file, idx), "This fragment class should be public ("+name+").")
	case file.FlatHasModifier(idx, "inner"):
		ctx.EmitAt(declLine(file, idx), declCol(file, idx), "This fragment inner class should be static ("+name+").")
	case ctorState.privateNoArgCtor != 0:
		ctor := ctorState.privateNoArgCtor
		ctx.EmitAt(file.FlatRow(ctor)+1, file.FlatCol(ctor)+1, "The default constructor of a fragment must be public.")
	case ctorState.hasParamCtor && !ctorState.hasNoArgCtor:
		ctx.EmitAt(declLine(file, idx), declCol(file, idx),
			"Fragment subclass must have a default (no-arg) constructor for framework re-instantiation.")
	}
}
