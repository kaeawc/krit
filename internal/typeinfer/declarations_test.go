package typeinfer

import "testing"

func TestIndexFileParallel_PrunesTraversalWithoutChangingSymbols(t *testing.T) {
	src := `
package com.example

class Outer {
    val outerValue: String = "x"

    companion object {
        fun outerFactory(): String = "factory"
        val outerFlag: Boolean = true
    }

    class Inner {
        fun innerValue(): String = "inner"
    }

    fun outerMethod(): Int = 1
}

fun topLevel(): String {
    fun localHelper(): String = "local"
    return localHelper()
}
`
	file := parseTestFile(t, src)
	fi := IndexFileParallel(file)
	if fi == nil {
		t.Fatal("expected FileTypeInfo")
	}

	if _, ok := fi.Functions["topLevel"]; !ok {
		t.Fatal("expected topLevel to be indexed")
	}
	if _, ok := fi.Functions["Outer.outerFactory"]; !ok {
		t.Fatal("expected companion object function to be indexed")
	}
	if _, ok := fi.Functions["Outer.outerFlag"]; !ok {
		t.Fatal("expected companion object property to be indexed")
	}
	var outer *ClassInfo
	for _, ci := range fi.Classes {
		if ci != nil && ci.Name == "Outer" {
			outer = ci
			break
		}
	}
	if outer == nil {
		t.Fatal("expected Outer class info to be indexed")
	}
	foundMethod := false
	for _, m := range outer.Members {
		if m.Name == "outerMethod" && m.Kind == "function" {
			foundMethod = true
			break
		}
	}
	if !foundMethod {
		t.Fatal("expected outerMethod to be indexed as a class member")
	}
	if fi.RootScope == nil {
		t.Fatal("expected RootScope")
	}
}
