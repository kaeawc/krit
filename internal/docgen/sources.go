// Package docgen writes one markdown page per registered rule, plus a
// category index, for the mkdocs site. URLs come from api.RuleDocsPath.
package docgen

import (
	"io/fs"
	"path"
	"regexp"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// Sources are the repo inputs the pages are built from.
type Sources struct {
	// Fixtures is rooted at tests/fixtures (positive/ and negative/ inside).
	Fixtures fs.FS
	// DetektDefaults is config/detekt-defaults.yml.
	DetektDefaults []byte
	// Migration is the text of docs/detekt-migration.md.
	Migration string
}

// detektInfo is what the detekt defaults and migration guide say about rules.
type detektInfo struct {
	rulesets map[string]string   // detekt rule name -> detekt ruleset
	renamed  map[string][]string // krit rule ID -> detekt names it replaces
	parity   map[string][]string // rule ID -> parity note bullets
}

var (
	renameRow  = regexp.MustCompile("^\\|\\s*`([A-Za-z0-9]+)`\\s*\\|\\s*(.+?)\\s*\\|\\s*$")
	backticked = regexp.MustCompile("`([A-Za-z][A-Za-z0-9]*)`")
	leadingID  = regexp.MustCompile("^`?([A-Z][A-Za-z0-9]*)`?(\\s|,|$)")
)

func loadDetektInfo(src Sources) detektInfo {
	info := detektInfo{
		rulesets: map[string]string{},
		renamed:  map[string][]string{},
		parity:   map[string][]string{},
	}
	var doc map[string]map[string]any
	if err := yaml.Unmarshal(src.DetektDefaults, &doc); err == nil {
		for set, rules := range doc {
			for name := range rules {
				if name != "active" && name != "excludes" && name != "includes" {
					info.rulesets[name] = set
				}
			}
		}
	}
	info.readMigration(src.Migration)
	return info
}

// readMigration collects the rename table and the per-rule bullet notes.
func (d *detektInfo) readMigration(text string) {
	lines := strings.Split(text, "\n")
	inRename := false
	for _, ln := range lines {
		switch {
		case strings.HasPrefix(ln, "| detekt rule |"):
			inRename = true
		case inRename && strings.HasPrefix(ln, "|"):
			d.addRename(ln)
		default:
			inRename = false
		}
	}
	for _, b := range bullets(lines) {
		if m := leadingID.FindStringSubmatch(b); m != nil {
			d.parity[m[1]] = append(d.parity[m[1]], b)
		}
	}
}

func (d *detektInfo) addRename(row string) {
	m := renameRow.FindStringSubmatch(row)
	if m == nil {
		return
	}
	for _, t := range backticked.FindAllStringSubmatch(m[2], -1) {
		d.renamed[t[1]] = append(d.renamed[t[1]], m[1])
	}
}

// bullets returns each top-level "- " list item with continuation lines joined.
func bullets(lines []string) []string {
	var out []string
	var cur []string
	flush := func() {
		if len(cur) > 0 {
			out = append(out, strings.Join(cur, " "))
			cur = nil
		}
	}
	for _, ln := range lines {
		switch {
		case strings.HasPrefix(ln, "- "):
			flush()
			cur = []string{strings.TrimSpace(ln[2:])}
		case cur != nil && strings.HasPrefix(ln, "  ") && strings.TrimSpace(ln) != "":
			cur = append(cur, strings.TrimSpace(ln))
		default:
			flush()
		}
	}
	flush()
	return out
}

// fixtureIndex maps "<kind>/<category>/<ID>" to the fixture path.
type fixtureIndex map[string]string

var fixtureExts = []string{".kt", ".kts", ".java", ".xml"}

func indexFixtures(fsys fs.FS) fixtureIndex {
	idx := fixtureIndex{}
	if fsys == nil {
		return idx
	}
	for _, kind := range []string{"positive", "negative"} {
		_ = fs.WalkDir(fsys, kind, func(p string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return nil //nolint:nilerr // skip-and-continue: an unreadable fixture is left out of the index
			}
			ext := path.Ext(p)
			for _, e := range fixtureExts {
				if ext == e {
					idx[strings.TrimSuffix(p, ext)] = p
				}
			}
			return nil
		})
	}
	return idx
}

// find returns the fixture path for a rule in kind, or "".
func (f fixtureIndex) find(kind, category, id string) string {
	return f[kind+"/"+category+"/"+id]
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
