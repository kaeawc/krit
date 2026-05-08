package rules

import (
	"sort"
	"strings"
	"testing"

	api "github.com/kaeawc/krit/internal/rules/api"
)

// TestPrintOracleFactBitsMigrationReport emits the bits each rule
// should add to its Needs declaration so the legacy ruleOracleFactBits
// lift can be deleted in Phase 4. Prints only rules whose declared
// Needs do not already match the bit union derived from their legacy
// metadata.
//
// Run with `-v -run TestPrintOracleFactBitsMigrationReport`.
func TestPrintOracleFactBitsMigrationReport(t *testing.T) {
	var rows []string
	for _, r := range api.Registry {
		if r == nil {
			continue
		}
		bits := ruleOracleFactBits(r)
		if bits == 0 {
			continue
		}
		declared := r.Needs & api.NeedsOracle
		missing := bits & ^declared
		if missing == 0 {
			continue
		}
		rows = append(rows, formatBitRow(r.ID, bits, declared, missing))
	}
	if len(rows) == 0 {
		return
	}
	sort.Strings(rows)
	t.Log("rules needing explicit bits (id | needed | declared | missing):")
	for _, row := range rows {
		t.Log(row)
	}
}

func formatBitRow(id string, needed, declared, missing api.Capabilities) string {
	return id + " | " +
		bitNames(needed) + " | " +
		bitNames(declared) + " | " +
		bitNames(missing)
}

func bitNames(c api.Capabilities) string {
	if c == 0 {
		return "—"
	}
	var names []string
	add := func(b api.Capabilities, name string) {
		if c&b == b && b != 0 {
			names = append(names, name)
		}
	}
	add(api.NeedsOracleCallTargets, "CT")
	add(api.NeedsOracleSuspendMarkers, "SM")
	add(api.NeedsOracleExprType, "ET")
	add(api.NeedsOracleExprAnnotations, "EA")
	add(api.NeedsOracleSupertypes, "ST")
	add(api.NeedsOracleMembers, "MB")
	add(api.NeedsOracleMemberSignatures, "MS")
	add(api.NeedsOracleClassAnnotations, "CA")
	add(api.NeedsOracleMemberAnnotations, "MA")
	add(api.NeedsOracleDiagnostics, "DG")
	add(api.NeedsOracleLibraryClasses, "LC")
	if len(names) == 0 {
		return "0x" + capabilitiesHex(c)
	}
	return strings.Join(names, "+")
}

func capabilitiesHex(c api.Capabilities) string {
	const hex = "0123456789abcdef"
	if c == 0 {
		return "0"
	}
	var b []byte
	for c > 0 {
		b = append([]byte{hex[c&0xf]}, b...)
		c >>= 4
	}
	return string(b)
}
