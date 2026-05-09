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

func TestExtractMembersFlat_PopulatesFunctionParams(t *testing.T) {
	src := `
package com.example

class Holder {
    fun greet(name: String, count: Int = 1, greeting: String?): String {
        return "hi"
    }

    fun <T : Any, R> transform(input: T): R = TODO()

    val identifier: Int = 42
}
`
	file := parseTestFile(t, src)
	fi := IndexFileParallel(file)
	if fi == nil {
		t.Fatal("expected FileTypeInfo")
	}
	var holder *ClassInfo
	for _, ci := range fi.Classes {
		if ci != nil && ci.Name == "Holder" {
			holder = ci
			break
		}
	}
	if holder == nil {
		t.Fatal("expected Holder class")
	}
	var greet, transform, identifier *MemberInfo
	for i := range holder.Members {
		m := &holder.Members[i]
		switch m.Name {
		case "greet":
			greet = m
		case "transform":
			transform = m
		case "identifier":
			identifier = m
		}
	}
	if greet == nil {
		t.Fatal("expected greet member")
	}
	if len(greet.Params) != 3 {
		t.Fatalf("greet expected 3 params, got %d (%+v)", len(greet.Params), greet.Params)
	}
	if greet.Params[0].Name != "name" || greet.Params[0].Type == nil || greet.Params[0].Type.Name != "String" {
		t.Errorf("greet param 0 = %+v, want name:String", greet.Params[0])
	}
	if !greet.Params[1].HasDefault {
		t.Errorf("greet param 1 (count) HasDefault = false, want true")
	}
	if greet.Params[2].Type == nil || !greet.Params[2].Type.Nullable {
		t.Errorf("greet param 2 (greeting) Nullable = false, want true")
	}

	if transform == nil {
		t.Fatal("expected transform member")
	}
	if got := transform.TypeParameters; len(got) != 2 || got[0] != "T" || got[1] != "R" {
		t.Errorf("transform.TypeParameters = %v, want [T R]", got)
	}

	if identifier == nil {
		t.Fatal("expected identifier property")
	}
	if identifier.Params != nil {
		t.Errorf("property.Params should be nil, got %+v", identifier.Params)
	}
}
