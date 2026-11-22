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
