package rules_test

import "testing"

func TestNullSafetyAbsenceClaimsRequireResolvedTypes(t *testing.T) {
	tests := []struct {
		name       string
		rule       string
		unresolved string
		resolved   string
	}{
		{
			name: "unnecessary null check",
			rule: "UnnecessaryNotNullCheck",
			unresolved: `package test
fun check(value: ExternalThing) {
    if (value != null) println(value)
}`,
			resolved: `package test
class KnownThing
fun check(value: KnownThing) {
    if (value != null) println(value)
}`,
		},
		{
			name: "unnecessary not-null operator",
			rule: "UnnecessaryNotNullOperator",
			unresolved: `package test
fun check(value: ExternalThing) {
    println(value!!)
}`,
			resolved: `package test
class KnownThing
fun check(value: KnownThing) {
    println(value!!)
}`,
		},
		{
			name: "unnecessary safe call",
			rule: "UnnecessarySafeCall",
			unresolved: `package test
fun check(value: ExternalThing) {
    println(value?.toString())
}`,
			resolved: `package test
class KnownThing
fun check(value: KnownThing) {
    println(value?.toString())
}`,
		},
		{
			name: "useless elvis",
			rule: "UselessElvisOnNonNull",
			unresolved: `package test
fun check(value: ExternalThing, fallback: ExternalThing): ExternalThing {
    return value ?: fallback
}`,
			resolved: `package test
class KnownThing
fun check(value: KnownThing, fallback: KnownThing): KnownThing {
    return value ?: fallback
}`,
		},
		{
			name: "nullable cast to non-null target",
			rule: "CastNullableToNonNullableType",
			unresolved: `package test
fun check(value: String?) {
    println(value as ExternalThing)
}`,
			resolved: `package test
class KnownThing
fun check(value: String?) {
    println(value as KnownThing)
}`,
		},
		{
			name: "of-not-null factory",
			rule: "UselessCallOnNotNull",
			unresolved: `package test
fun check(value: ExternalThing) {
    println(listOfNotNull(value))
}`,
			resolved: `package test
class KnownThing
fun check(value: KnownThing) {
    println(listOfNotNull(value))
}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if findings := runRuleByNameWithResolver(t, tt.rule, tt.unresolved); len(findings) != 0 {
				t.Fatalf("unresolved type produced %d findings: %v", len(findings), findings)
			}
			if findings := runRuleByNameWithResolver(t, tt.rule, tt.resolved); len(findings) == 0 {
				t.Fatal("genuinely resolved non-null type produced no finding")
			}
		})
	}
}
