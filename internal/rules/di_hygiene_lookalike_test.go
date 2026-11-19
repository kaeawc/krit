package rules_test

import (
	"testing"
)

// The Go fallback of the DI-binding rules reads annotation names, so a same-named annotation the file
// imports from another package, or declares itself, must not count as the DI framework's.
func TestDIBindingRulesSkipLookalikeAnnotations(t *testing.T) {
	cases := []struct {
		name  string
		rule  string
		code  string
		lines []int
	}{
		{
			name: "Binds imported from another package",
			rule: "BindsReturnTypeMatchesParam",
			code: `package demo

import com.example.Binds

interface Foo

interface M {
    @Binds fun same(foo: Foo): Foo
    @dagger.Binds fun qualified(foo: Foo): Foo
}
`,
			lines: []int{9},
		},
		{
			name: "Binds declared in the file",
			rule: "BindsReturnTypeMatchesParam",
			code: `package demo

annotation class Binds

interface Foo

interface M {
    @Binds fun same(foo: Foo): Foo
}
`,
		},
		{
			name: "Binds import only in a header comment",
			rule: "BindsReturnTypeMatchesParam",
			code: `package demo

/*
import com.example.Binds
*/
import dagger.Binds

interface Foo

interface M {
    @Binds fun same(foo: Foo): Foo
}
`,
			lines: []int{11},
		},
		{
			name: "Binds declaration only in a comment or a string",
			rule: "BindsReturnTypeMatchesParam",
			code: `package demo

import dagger.*

// annotation class Binds
val note = "annotation class Binds"

interface Foo

interface M {
    @Binds fun same(foo: Foo): Foo
}
`,
			lines: []int{11},
		},
		{
			name: "Dagger Binds imported despite a local declaration",
			rule: "BindsReturnTypeMatchesParam",
			code: `package demo

import dagger.Binds

annotation class Other

interface Foo

interface M {
    @Binds fun same(foo: Foo): Foo
}
`,
			lines: []int{10},
		},
		{
			name: "Binds between differently qualified keys or into a multibinding",
			rule: "BindsReturnTypeMatchesParam",
			code: `package demo

import dagger.Binds
import dagger.multibindings.IntoSet
import javax.inject.Named

interface Foo

interface M {
    @Named("a") @Binds fun qualifiedReturn(foo: Foo): Foo
    @Binds fun qualifiedParameter(@Named("b") foo: Foo): Foo
    @Named("a") @Binds fun differentQualifiers(@Named("b") foo: Foo): Foo
    @Binds @IntoSet fun intoSet(foo: Foo): Foo
    @javax.inject.Named("a") @Binds @Suppress("x") fun sameQualifier(@Named( "a" ) foo: Foo): Foo
    @Binds @JvmSuppressWildcards fun plain(foo: Foo): Foo
}
`,
			lines: []int{14, 15},
		},
		{
			name: "IntoMap imported from another package",
			rule: "IntoMapMissingKey",
			code: `package demo

import dagger.Provides
import com.example.IntoMap

interface Foo

object M {
    @Provides @IntoMap fun noKey(): Foo = TODO()
    @Provides @dagger.multibindings.IntoMap fun qualified(): Foo = TODO()
}
`,
			lines: []int{10},
		},
		{
			name: "IntoMap aliased from another class",
			rule: "IntoMapMissingKey",
			code: `package demo

import dagger.Provides
import com.example.Other as IntoMap

interface Foo

object M {
    @Provides @IntoMap fun noKey(): Foo = TODO()
}
`,
		},
		{
			name: "IntoSet and Provides declared in the file",
			rule: "IntoSetOnNonSetReturn",
			code: `package demo

annotation class Provides
annotation class IntoSet

interface Foo

object M {
    @Provides @IntoSet fun list(): List<Foo> = emptyList()
}
`,
		},
		{
			name: "IntoSet with Provides from another package",
			rule: "IntoSetOnNonSetReturn",
			code: `package demo

import com.example.Provides
import dagger.multibindings.IntoSet

interface Foo

object M {
    @Provides @IntoSet fun list(): List<Foo> = emptyList()
    @dagger.Provides @IntoSet fun qualified(): List<Foo> = emptyList()
}
`,
			lines: []int{10},
		},
		{
			name: "IntoSet from kotlin-inject",
			rule: "IntoSetOnNonSetReturn",
			code: `package demo

import me.tatarka.inject.annotations.IntoSet
import me.tatarka.inject.annotations.Provides

interface Foo

abstract class C {
    @Provides @IntoSet fun list(): List<Foo> = emptyList()
}
`,
			lines: []int{9},
		},
		{
			name: "scope named in a comment between annotations",
			rule: "ScopeOnParameterizedClass",
			code: `package demo

@Deprecated("x")
// was @Singleton
@Suppress("unused")
class Cache<T>
`,
		},
		{
			name: "local scope lookalike, qualified real scope, local real scope",
			rule: "ScopeOnParameterizedClass",
			code: `package demo

import javax.inject.Scope

annotation class Singleton

@Scope
@Retention(AnnotationRetention.RUNTIME)
annotation class UserScoped

@Singleton class Local<T>
@javax.inject.Singleton class Qualified<T>
@UserScoped class Project<T>
`,
			lines: []int{12, 13},
		},
		{
			name: "imported scope shadows nothing",
			rule: "ScopeOnParameterizedClass",
			code: `package demo

import javax.inject.Singleton

annotation class Other

@Singleton class Imported<T>
`,
			lines: []int{7},
		},
		{
			name: "unmarked scope declared in its own package",
			rule: "ScopeOnParameterizedClass",
			code: `package org.koin.core.annotation

annotation class Singleton

@Singleton class KoinBean<T>
`,
			lines: []int{5},
		},
	}
	index := buildRuleIndex()
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rule := index[tc.rule]
			if rule == nil {
				t.Fatalf("%s not registered", tc.rule)
			}
			var got []int
			for _, f := range runRule(t, rule, parseInline(t, tc.code)) {
				got = append(got, f.Line)
			}
			if len(got) != len(tc.lines) {
				t.Fatalf("lines = %v, want %v", got, tc.lines)
			}
			for i := range got {
				if got[i] != tc.lines[i] {
					t.Fatalf("lines = %v, want %v", got, tc.lines)
				}
			}
		})
	}
}
