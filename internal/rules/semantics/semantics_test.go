package semantics

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/kaeawc/krit/internal/oracle"
	v2 "github.com/kaeawc/krit/internal/rules/v2"
	"github.com/kaeawc/krit/internal/scanner"
	"github.com/kaeawc/krit/internal/typeinfer"
)

func TestResolveCallTargetUsesOracleAndStructuredArguments(t *testing.T) {
	file := parseKotlin(t, `package com.example

fun bad(map: Map<String, String?>, key: String) {
    val value = map.get(
        key
    )!!
}
`)
	call := findCall(t, file, "get")
	fake := oracle.NewFakeOracle()
	fake.CallTargets[file.Path] = map[string]string{
		posKey(file, call): "kotlin.collections.Map.get",
	}
	ctx := &v2.Context{
		File:     file,
		Resolver: oracle.NewCompositeResolver(fake, typeinfer.NewFakeResolver()),
	}

	target, ok := ResolveCallTarget(ctx, call)
	if !ok {
		t.Fatal("ResolveCallTarget returned ok=false")
	}
	if !target.Resolved || target.QualifiedName != "kotlin.collections.Map.get" || target.CalleeName != "get" {
		t.Fatalf("target = %+v, want resolved kotlin.collections.Map.get", target)
	}
	if !target.Receiver.Valid() || file.FlatNodeText(target.Receiver.Node) != "map" {
		t.Fatalf("receiver = %+v, want map", target.Receiver)
	}
	if len(target.Arguments) != 1 || file.FlatNodeText(target.Arguments[0].Node) != "key" {
		t.Fatalf("arguments = %+v, want one key argument", target.Arguments)
	}
	if !IsResolvedCall(ctx, call, "kotlin.collections.Map.get") {
		t.Fatal("IsResolvedCall did not match resolved Map.get")
	}
}

func TestResolvedCallSkipsUnresolvedAndUnrelatedSameName(t *testing.T) {
	file := parseKotlin(t, `package com.example

class Fake {
    fun get(key: String): String? = null
}

fun ok(fake: Fake) {
    val value = fake.get("x")!!
}
`)
	call := findCall(t, file, "get")
	ctx := &v2.Context{File: file}
	if target, ok := ResolveCallTarget(ctx, call); !ok || target.Resolved {
		t.Fatalf("ResolveCallTarget without oracle = %+v, %v; want lexical unresolved", target, ok)
	}
	if IsResolvedCall(ctx, call, "kotlin.collections.Map.get") {
		t.Fatal("unresolved lexical get matched Map.get")
	}

	fake := oracle.NewFakeOracle()
	fake.CallTargets[file.Path] = map[string]string{
		posKey(file, call): "com.example.Fake.get",
	}
	ctx.Resolver = oracle.NewCompositeResolver(fake, typeinfer.NewFakeResolver())
	if IsResolvedCall(ctx, call, "kotlin.collections.Map.get") {
		t.Fatal("resolved Fake.get matched Map.get")
	}
}

func TestSameFileDeclarationMatchIgnoresCommentsStringsAndShadowing(t *testing.T) {
	file := parseKotlin(t, `package com.example

class A {
    fun target() = Unit
}

class B {
    fun target() = Unit

    fun run(id: String) {
        val guid = "not a use of id or target()"
        // target()
        println(id)
        target()
    }
}
`)
	ctx := &v2.Context{File: file}
	aTarget := findFunctionDecl(t, file, "A", "target")
	bTarget := findFunctionDecl(t, file, "B", "target")
	targetCall := findCallInOwner(t, file, "B", "target")
	if SameFileDeclarationMatch(ctx, aTarget, targetCall) {
		t.Fatal("A.target matched B.target() call")
	}
	if !SameFileDeclarationMatch(ctx, bTarget, targetCall) {
		t.Fatal("B.target did not match B.target() call")
	}

	param := findParameterDecl(t, file, "id")
	idRef := findIdentifierAfter(t, file, "id", "println")
	if !SameFileDeclarationMatch(ctx, param, idRef) {
		t.Fatal("parameter id did not match println(id) reference")
	}

	realTargetRefs := 0
	file.FlatWalkAllNodes(0, func(idx uint32) {
		if file.FlatType(idx) == "call_expression" && referenceName(file, idx) == "target" {
			realTargetRefs++
		}
	})
	if realTargetRefs != 1 {
		t.Fatalf("target call refs = %d, want 1 real call and no string/comment refs", realTargetRefs)
	}
}

func TestMatchQualifiedReceiverHandlesImportAliases(t *testing.T) {
	file := parseKotlin(t, `package com.example

import android.animation.ObjectAnimator as Animator

fun animate(target: Any) {
    Animator.ofFloat(target, "alpha", 1f)
}
`)
	call := findCall(t, file, "ofFloat")
	resolver := typeinfer.NewResolver()
	resolver.IndexFilesParallel([]*scanner.File{file}, 1)
	ctx := &v2.Context{File: file, Resolver: resolver}
	if !MatchQualifiedReceiver(ctx, call, "android.animation.ObjectAnimator") {
		t.Fatal("import alias receiver did not match ObjectAnimator")
	}
	if MatchQualifiedReceiver(ctx, call, "android.animation.ValueAnimator") {
		t.Fatal("ObjectAnimator alias matched unrelated ValueAnimator")
	}
}

func TestEvalConstSameFileConstAndInterpolatedString(t *testing.T) {
	file := parseKotlin(t, `package com.example

const val PERMISSION =
    "android.permission.READ_CONTACTS"

fun request() {
    val interpolated = "android.permission.$PERMISSION"
    check(PERMISSION)
}
`)
	ctx := &v2.Context{File: file}
	ref := findIdentifierAfter(t, file, "PERMISSION", "check")
	val, ok := EvalSameFileConst(ctx, ref)
	if !ok || val.Kind != "string" || val.String != "android.permission.READ_CONTACTS" {
		t.Fatalf("EvalSameFileConst = %+v, %v", val, ok)
	}
	interpolated := findPropertyInitializer(t, file, "interpolated")
	if val, ok := EvalConst(ctx, interpolated); ok {
		t.Fatalf("interpolated string evaluated unexpectedly: %+v", val)
	}
}

func TestDominatingTypeFactsAndConfidencePolicy(t *testing.T) {
	file := parseKotlin(t, `package com.example

fun f(value: Any) {
    if (value is String) {
        println(value.length)
    }
}
`)
	ctx := &v2.Context{File: file}
	call := findCall(t, file, "println")
	facts := DominatingTypeFacts(ctx, call)
	if len(facts) != 1 || facts[0].TypeName != "String" || !facts[0].Positive {
		t.Fatalf("facts = %+v, want positive String fact", facts)
	}
	if conf, ok := ConfidenceForEvidence(0.95, EvidenceSameFileDeclaration); !ok || conf != 0.80 {
		t.Fatalf("same-file confidence = %v, %v; want 0.80 true", conf, ok)
	}
	if _, ok := ConfidenceForEvidence(0.95, EvidenceUnresolved); ok {
		t.Fatal("unresolved evidence should not be actionable by default")
	}
}

func parseKotlin(t *testing.T, content string) *scanner.File {
	t.Helper()
	path := filepath.Join(t.TempDir(), "Sample.kt")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	file, err := scanner.ParseFile(path)
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}
	return file
}

func posKey(file *scanner.File, idx uint32) string {
	return fmt.Sprintf("%d:%d", file.FlatRow(idx)+1, file.FlatCol(idx)+1)
}

func findCall(t *testing.T, file *scanner.File, name string) uint32 {
	t.Helper()
	var found uint32
	file.FlatWalkNodes(0, "call_expression", func(idx uint32) {
		if found == 0 && callExpressionName(file, idx) == name {
			found = idx
		}
	})
	if found == 0 {
		t.Fatalf("call %q not found", name)
	}
	return found
}

func findCallInOwner(t *testing.T, file *scanner.File, ownerName, callName string) uint32 {
	t.Helper()
	var found uint32
	file.FlatWalkNodes(0, "call_expression", func(idx uint32) {
		if found == 0 && callExpressionName(file, idx) == callName && declarationName(file, enclosingOwner(file, idx)) == ownerName {
			found = idx
		}
	})
	if found == 0 {
		t.Fatalf("call %s.%s not found", ownerName, callName)
	}
	return found
}

func findFunctionDecl(t *testing.T, file *scanner.File, ownerName, functionName string) uint32 {
	t.Helper()
	var found uint32
	file.FlatWalkNodes(0, "function_declaration", func(idx uint32) {
		if found == 0 && declarationName(file, idx) == functionName && declarationName(file, enclosingOwner(file, idx)) == ownerName {
			found = idx
		}
	})
	if found == 0 {
		t.Fatalf("function %s.%s not found", ownerName, functionName)
	}
	return found
}

func findParameterDecl(t *testing.T, file *scanner.File, name string) uint32 {
	t.Helper()
	var found uint32
	file.FlatWalkNodes(0, "parameter", func(idx uint32) {
		if found == 0 && declarationName(file, idx) == name {
			found = idx
		}
	})
	if found == 0 {
		t.Fatalf("parameter %q not found", name)
	}
	return found
}

func findIdentifierAfter(t *testing.T, file *scanner.File, name, after string) uint32 {
	t.Helper()
	afterAt := uint32(0)
	file.FlatWalkAllNodes(0, func(idx uint32) {
		if afterAt == 0 && file.FlatNodeText(idx) == after {
			afterAt = file.FlatEndByte(idx)
		}
	})
	if afterAt == 0 {
		t.Fatalf("anchor %q not found", after)
	}
	var found uint32
	file.FlatWalkAllNodes(0, func(idx uint32) {
		if found == 0 && file.FlatType(idx) == "simple_identifier" &&
			file.FlatNodeText(idx) == name && file.FlatStartByte(idx) > afterAt {
			found = idx
		}
	})
	if found == 0 {
		t.Fatalf("identifier %q after %q not found", name, after)
	}
	return found
}

func findPropertyInitializer(t *testing.T, file *scanner.File, name string) uint32 {
	t.Helper()
	var found uint32
	file.FlatWalkNodes(0, "property_declaration", func(idx uint32) {
		if found == 0 && declarationName(file, idx) == name {
			found = propertyInitializer(file, idx)
		}
	})
	if found == 0 {
		t.Fatalf("initializer for %q not found", name)
	}
	return found
}
