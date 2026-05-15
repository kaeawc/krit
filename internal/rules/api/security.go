package api

import "strings"

// SecurityTaxonomy carries structured identifiers from public security
// catalogs (CWE, OWASP Top 10, SEI CERT, MITRE ATT&CK) that classify a
// security rule. The struct is informational metadata: the dispatcher
// never keys behavior on these IDs. Consumers (SARIF taxa, MCP explain,
// CLI filters, dashboards) read them to render badges, link out to the
// canonical taxonomy entry, or filter rules by ID.
//
// All ID slices are case-preserving but case-insensitive at lookup time.
// Empty slices and a nil Security pointer are both legal: a nil pointer
// means "this rule has no published taxonomy mapping yet"; an empty
// slice on one axis means "the rule is mapped, but not to any IDs on
// that axis." Consumers should treat both the same way.
type SecurityTaxonomy struct {
	// CWE lists Common Weakness Enumeration IDs in the canonical
	// "CWE-<n>" form, e.g. "CWE-89" for SQL injection. The SARIF taxa
	// emitter uses the CWE catalog for the corresponding taxonomy
	// reference; GitHub code-scanning renders the chip from this ID.
	CWE []string

	// OWASP lists OWASP Top 10 category IDs, e.g. "A03:2021-Injection".
	// Year-prefixed values are preferred so the mapping survives top-10
	// re-orderings.
	OWASP []string

	// SEICert lists SEI CERT secure-coding rule IDs (Java or Android
	// where applicable), e.g. "IDS00-J" or "FIO52-J".
	SEICert []string

	// Mitre lists optional MITRE ATT&CK technique IDs, e.g. "T1059".
	// Most rules will leave this nil; supply when the rule maps to a
	// concrete adversary technique.
	Mitre []string
}

// IsEmpty reports whether the taxonomy contributes no IDs. A nil
// pointer is also considered empty; callers using TaxonomyMatcher
// rely on this to skip rules with no security mapping.
func (t *SecurityTaxonomy) IsEmpty() bool {
	if t == nil {
		return true
	}
	return len(t.CWE) == 0 && len(t.OWASP) == 0 && len(t.SEICert) == 0 && len(t.Mitre) == 0
}

// HasCWE reports whether t lists the given CWE ID. Comparison is
// case-insensitive and tolerates whitespace differences so config and
// CLI input do not have to match the canonical capitalization exactly.
func (t *SecurityTaxonomy) HasCWE(id string) bool {
	return t.has(t.CWE, id)
}

// HasOWASP reports whether t lists the given OWASP category ID.
func (t *SecurityTaxonomy) HasOWASP(id string) bool {
	return t.has(t.OWASP, id)
}

// HasSEICert reports whether t lists the given SEI CERT rule ID.
func (t *SecurityTaxonomy) HasSEICert(id string) bool {
	return t.has(t.SEICert, id)
}

// HasMitre reports whether t lists the given MITRE ATT&CK technique ID.
func (t *SecurityTaxonomy) HasMitre(id string) bool {
	return t.has(t.Mitre, id)
}

func (t *SecurityTaxonomy) has(ids []string, id string) bool {
	if t == nil {
		return false
	}
	target := normalizeTaxonomyID(id)
	if target == "" {
		return false
	}
	for _, candidate := range ids {
		if normalizeTaxonomyID(candidate) == target {
			return true
		}
	}
	return false
}

// TaxonomyMatcher matches rules by taxonomy ID. The dispatcher uses it
// for CLI filtering ("krit rules list --cwe CWE-79") and the SARIF
// emitter uses it to emit only relevant taxa references. Zero value
// matches no rules; populate IDs to opt in.
type TaxonomyMatcher struct {
	IDs []string
}

// Matches reports whether t lists any of the matcher's IDs on any axis.
// Empty matcher matches no rules. Nil taxonomy matches no rules.
func (m TaxonomyMatcher) Matches(t *SecurityTaxonomy) bool {
	if t == nil || len(m.IDs) == 0 {
		return false
	}
	for _, id := range m.IDs {
		if t.HasCWE(id) || t.HasOWASP(id) || t.HasSEICert(id) || t.HasMitre(id) {
			return true
		}
	}
	return false
}

func normalizeTaxonomyID(id string) string {
	return strings.ToUpper(strings.TrimSpace(id))
}
