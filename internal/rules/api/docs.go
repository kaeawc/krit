package api

import (
	"strings"
	"sync/atomic"
)

// DocsSiteURL is the root of the published documentation site.
const DocsSiteURL = "https://kaeawc.github.io/krit/"

// DefaultDocsBaseURL is the rules section of the documentation site. The
// default resolver builds "<base><category>/<rule-id-kebab>/" from it.
// Must end with "/".
const DefaultDocsBaseURL = DocsSiteURL + "rules/"

// DocsSlug converts a rule ID or category to its URL slug: lower-case
// kebab-case ("MagicNumber" -> "magic-number", "XMLParser" -> "xml-parser",
// "untested_public_api" -> "untested-public-api"). It is the only place the
// slug is computed; the docs generator uses it for page paths.
func DocsSlug(s string) string {
	runes := []rune(s)
	var b strings.Builder
	for i, c := range runes {
		switch {
		case c == '_' || c == ' ':
			b.WriteByte('-')
		case isUpperRune(c):
			if i > 0 && needsSlugBreak(runes, i) {
				b.WriteByte('-')
			}
			b.WriteRune(c + ('a' - 'A'))
		default:
			b.WriteRune(c)
		}
	}
	return b.String()
}

func isUpperRune(c rune) bool { return c >= 'A' && c <= 'Z' }

// needsSlugBreak reports whether an upper-case rune at i starts a new word:
// it follows a lower-case letter or digit, or ends an acronym run before a
// lower-case letter.
func needsSlugBreak(runes []rune, i int) bool {
	prev := runes[i-1]
	if prev == '_' || prev == ' ' || prev == '-' {
		return false
	}
	if !isUpperRune(prev) {
		return true
	}
	return i+1 < len(runes) && runes[i+1] >= 'a' && runes[i+1] <= 'z'
}

// RuleDocsPath returns the docs-site path of a rule page relative to the
// site root, e.g. "rules/style/magic-number/". It is empty without an ID.
func RuleDocsPath(category, id string) string {
	if id == "" {
		return ""
	}
	if category == "" {
		category = "other"
	}
	return "rules/" + DocsSlug(category) + "/" + DocsSlug(id) + "/"
}

// RuleDocsURLFor returns the stable documentation URL for a rule in the
// given category: DocsSiteURL + RuleDocsPath. Empty without an ID.
func RuleDocsURLFor(category, id string) string {
	p := RuleDocsPath(category, id)
	if p == "" {
		return ""
	}
	return DocsSiteURL + p
}

// RuleDocsURLByID resolves a rule ID, or one of its aliases, against the
// registry and returns the canonical docs URL. Unknown IDs return "".
func RuleDocsURLByID(id string) string {
	for _, r := range Registry {
		if r.ID == id {
			return RuleDocsURL(r)
		}
	}
	for _, r := range Registry {
		for _, a := range r.Aliases {
			if a == id {
				return RuleDocsURL(r)
			}
		}
	}
	return ""
}

// DocsResolver derives the documentation URL for a rule.
type DocsResolver interface {
	DocsURL(r *Rule) string
}

// DefaultDocsResolver returns Rule.DocsURL when set. Otherwise a zero value
// returns RuleDocsURLFor(category, id); a non-empty BaseURL (tests) yields
// "<BaseURL><category>/<rule-id-kebab>/". BaseURL must end with "/".
type DefaultDocsResolver struct {
	BaseURL string
}

// DocsURL implements DocsResolver.
func (d DefaultDocsResolver) DocsURL(r *Rule) string {
	if r == nil {
		return ""
	}
	if r.DocsURL != "" {
		return r.DocsURL
	}
	if r.ID == "" {
		return ""
	}
	if d.BaseURL == "" {
		return RuleDocsURLFor(r.Category, r.ID)
	}
	return d.BaseURL + strings.TrimPrefix(RuleDocsPath(r.Category, r.ID), "rules/")
}

var docsResolver atomic.Pointer[DocsResolver]

func init() {
	var d DocsResolver = DefaultDocsResolver{}
	docsResolver.Store(&d)
}

// RuleDocsURL returns the canonical documentation URL for r.
func RuleDocsURL(r *Rule) string {
	return (*docsResolver.Load()).DocsURL(r)
}

// SetDocsResolver swaps the package-level resolver and returns the
// prior value. Passing nil installs the zero-value DefaultDocsResolver.
// Safe for concurrent use.
func SetDocsResolver(d DocsResolver) DocsResolver {
	if d == nil {
		d = DefaultDocsResolver{}
	}
	prev := docsResolver.Swap(&d)
	if prev == nil {
		return DefaultDocsResolver{}
	}
	return *prev
}
