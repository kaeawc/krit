package rules_test

import (
	"testing"

	"github.com/kaeawc/krit/internal/rules"
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
	"github.com/kaeawc/krit/internal/typeinfer"
)

func TestNullSafetyNullableMemberDoesNotInheritUnrelatedConstructorLocal(t *testing.T) {
	const declarations = `
package example
import example.model.Analytics
sealed class Key {
    class Conv(val analytics: Analytics?) : Key()
    object Other : Key()
}
fun Intent.build(): Analytics {
    val analytics = Analytics(type = "x")
    return analytics
}
`
	for _, test := range []struct {
		rule string
		body string
	}{
		{"UnnecessarySafeCall", `fun Key.resolve(): String? = when (this) {
    is Key.Conv -> analytics?.type
    else -> null
}`},
		{"UselessElvisOnNonNull", `fun Key.resolve(): Analytics? = when (this) {
    is Key.Conv -> analytics ?: null
    else -> null
}`},
		{"UnnecessaryNotNullOperator", `fun Key.resolve(): Analytics? = when (this) {
    is Key.Conv -> analytics!!
    else -> null
}`},
	} {
		t.Run(test.rule, func(t *testing.T) {
			model := parseInline(t, `package example.model
class Analytics(val type: String? = null) {
    fun copy(type: String?) = Analytics(type)
}`)
			file := parseInline(t, declarations+test.body)
			resolver := typeinfer.NewResolver()
			resolver.IndexFilesParallel([]*scanner.File{model, file}, 1)
			var selected []*api.Rule
			for _, candidate := range api.Registry {
				if candidate.ID == test.rule {
					selected = append(selected, candidate)
				}
			}
			if len(selected) != 1 {
				t.Fatalf("expected one registered rule for %s, got %d", test.rule, len(selected))
			}
			columns := rules.NewDispatcher(selected, resolver).Run(file)
			if findings := columns.Findings(); len(findings) != 0 {
				t.Fatalf("nullable implicit receiver member must remain silent: %+v", findings)
			}
		})
	}
}

// Regression for #1673: inside a property initializer the primary-constructor
// parameter shadows a same-named member property. Another function in the
// class with a same-named parameter used to flip resolution to the non-null
// member and report the needed elvis as useless.
func TestUselessElvisOnNonNull_ConstructorParameterShadowsMemberInInitializer(t *testing.T) {
	for _, test := range []struct {
		name  string
		other string
	}{
		{"no other function", ""},
		{"other function parameter", "    fun other(id: Long?) = id\n"},
		{"copy function default", "    fun copy(id: Long? = this.id) = PendingModel(id)\n"},
	} {
		t.Run(test.name, func(t *testing.T) {
			findings := runRuleByNameWithResolver(t, "UselessElvisOnNonNull", `package demo

class Row(val id: Long)

class PendingModel(id: Long? = null) {
    private val delegate: Row = Row(id ?: -1)
    private val fromInit: Long

    init {
        fromInit = id ?: -2
    }

    val id: Long
        get() = delegate.id

`+test.other+`}
`)
			if len(findings) != 0 {
				t.Fatalf("constructor parameter shadows the member in initializers; expected no findings, got %+v", findings)
			}
		})
	}

	t.Run("same-named property initialized from the parameter", func(t *testing.T) {
		findings := runRuleByNameWithResolver(t, "UselessElvisOnNonNull", `package demo

class Model(id: Long?, private val name: String?) {
    val id: Long = id ?: 0
    val label: String by lazy { name ?: "none" }
}
`)
		if len(findings) != 0 {
			t.Fatalf("expected no findings, got %+v", findings)
		}
	})

	// #1727 review: a comment between `=` and the initializer, and enum
	// classes, which have an enum_class_body.
	t.Run("comment before the initializer and enum classes", func(t *testing.T) {
		findings := runRuleByNameWithResolver(t, "UselessElvisOnNonNull", `package demo

class Row(val id: Long)

class Commented(id: Long? = null) {
    private val delegate: Row = // keep the fallback
        Row(id ?: -1)
    private val block: Row = /* fallback */ Row(id ?: -2)

    val id: Long
        get() = delegate.id

    fun other(id: Long?) = id
}

enum class Kind(id: Long?) {
    A(1), B(null);

    val stored: Long = id ?: 0
    val id: Long get() = stored

    fun other(id: Long?) = id
}
`)
		if len(findings) != 0 {
			t.Fatalf("expected no findings, got %+v", findings)
		}
	})

	t.Run("member access in a function body still resolves to the property", func(t *testing.T) {
		findings := runRuleByNameWithResolver(t, "UselessElvisOnNonNull", `package demo

class Row(val id: Long)

class PendingModel(id: Long? = null) {
    private val delegate: Row = Row(id ?: -1)

    val id: Long
        get() = delegate.id

    fun other(id: Long?) = id

    fun read(): Long = id ?: 0
}
`)
		if len(findings) != 1 {
			t.Fatalf("expected the member-property elvis in read() to be flagged, got %+v", findings)
		}
	})
}
