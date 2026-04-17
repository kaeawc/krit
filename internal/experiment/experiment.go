package experiment

import (
	"fmt"
	"sort"
	"strings"
	"sync"
)

type Definition struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Intent      string   `json:"intent,omitempty"`
	TargetRules []string `json:"targetRules,omitempty"`
	// Status is the lifecycle state of the experiment:
	//   "" or "experimental" — flag-gated, off by default (default behavior).
	//   "promoted"           — enabled by default; opt-out via -experiment-off=name.
	//   "deprecated"         — superseded/removed; kept for documentation. Warns
	//                          and is ignored if a user tries to enable it.
	Status string `json:"status,omitempty"`
}

// Experiment status lifecycle constants.
const (
	StatusExperimental = "experimental"
	StatusPromoted     = "promoted"
	StatusDeprecated   = "deprecated"
)

// DefaultEnabled returns the names of experiments whose Status is "promoted".
// These are merged into the active set before user flags unless explicitly
// disabled via -experiment-off=name.
func DefaultEnabled() []string {
	var out []string
	for _, def := range knownDefinitions {
		if def.Status == StatusPromoted {
			out = append(out, def.Name)
		}
	}
	sort.Strings(out)
	return out
}

// IsPromoted reports whether the named experiment's Status is "promoted".
func IsPromoted(name string) bool {
	for _, def := range knownDefinitions {
		if def.Name == name {
			return def.Status == StatusPromoted
		}
	}
	return false
}

// IsDeprecated reports whether the named experiment's Status is "deprecated".
func IsDeprecated(name string) bool {
	for _, def := range knownDefinitions {
		if def.Name == name {
			return def.Status == StatusDeprecated
		}
	}
	return false
}

// Lookup returns the Definition for name and whether it exists.
func Lookup(name string) (Definition, bool) {
	for _, def := range knownDefinitions {
		if def.Name == name {
			return def, true
		}
	}
	return Definition{}, false
}

var knownDefinitions = []Definition{
	{Name: "no-name-shadowing-prune", Description: "Experimental traversal pruning for NoNameShadowing.", Intent: "performance", TargetRules: []string{"NoNameShadowing"}},
	{Name: "magic-number-ancestor-scan", Description: "Experimental single-pass ancestor scan for MagicNumber context checks.", Intent: "performance", TargetRules: []string{"MagicNumber"}},
	{Name: "unnecessary-safe-call-local-nullability", Description: "Experimental local-nullability shortcuts for UnnecessarySafeCall.", Intent: "fp-reduction", TargetRules: []string{"UnnecessarySafeCall"}},
	{Name: "unnecessary-safe-call-structural", Description: "Experimental structural receiver checks for UnnecessarySafeCall.", Intent: "fp-reduction", TargetRules: []string{"UnnecessarySafeCall"}},
	{Name: "exceptions-allowlist-cache", Description: "Experimental cached allowlist lookup for exception-message rules.", Intent: "performance", TargetRules: []string{"ThrowingExceptionsWithoutMessageOrCause"}},
	{Name: "exceptions-throw-fastpath", Description: "Experimental direct child fast path for throw-without-message checks.", Intent: "performance", TargetRules: []string{"ThrowingExceptionsWithoutMessageOrCause"}},
	{Name: "no-name-shadowing-skip-destructuring", Description: "Skip NoNameShadowing for components of val/lambda destructuring declarations (detekt parity).", Intent: "fp-reduction", TargetRules: []string{"NoNameShadowing"}},
	{Name: "exported-without-permission-skip-system-actions", Description: "Skip ExportedWithoutPermission for components that declare well-known public intent actions (SEND, VIEW deep links, sync/account/job services).", Intent: "fp-reduction", TargetRules: []string{"ExportedWithoutPermission"}},
	{Name: "string-not-localizable-default-values-only", Description: "Only fire StringNotLocalizableResource on the default res/values/strings.xml — translation overrides (values-xx/, values-night/, etc.) should not be flagged.", Intent: "fp-reduction", TargetRules: []string{"StringNotLocalizableResource"}},
	{Name: "naming-allow-backing-properties", Description: "Skip ObjectPropertyNaming / TopLevelPropertyNaming for private properties with a leading underscore — this is the idiomatic Kotlin backing-property convention (private val _foo + public val foo).", Intent: "fp-reduction", TargetRules: []string{"ObjectPropertyNaming", "TopLevelPropertyNaming"}},
	{Name: "swallowed-exception-broader-logging", Description: "Treat common free-function logging/warning calls (warn/error/info/debug + Timber/logger.*) as handling in SwallowedException, not just Log.v/d/i/w/e.", Intent: "fp-reduction", TargetRules: []string{"SwallowedException"}},
	{Name: "disable-baseline-alignment-require-text-children", Description: "Only fire DisableBaselineAlignmentResource when the LinearLayout has at least one direct text-displaying child (TextView/EditText/Button/CheckBox/RadioButton/TextInputEditText). Layouts whose weighted children are containers (nested LinearLayout/FrameLayout, ImageView) have negligible baseline-alignment overhead.", Intent: "fp-reduction", TargetRules: []string{"DisableBaselineAlignmentResource"}},
	{Name: "required-size-skip-implicit-dimensions", Description: "Skip RequiredSizeResource for view types whose layout dimensions are implicit (TableRow inside TableLayout gets match_parent/wrap_content automatically).", Intent: "fp-reduction", TargetRules: []string{"RequiredSizeResource"}},
	{Name: "orientation-resource-skip-fixed-height-rows", Description: "Skip OrientationResource when the LinearLayout has a fixed-dp / ?attr height that signals a horizontal button-row layout. The missing android:orientation is the intentional default for these row patterns.", Intent: "fp-reduction", TargetRules: []string{"OrientationResource"}},
	{Name: "magic-number-skip-regex-group-indices", Description: "Skip MagicNumber on integer literals passed as arguments to Matcher/MatchResult group accessors (`matcher.group(N)`, `match.groupValues[N]`, `matches.range(N)`). Regex capture group indices are intrinsic to the pattern, not extractable constants.", Intent: "fp-reduction", TargetRules: []string{"MagicNumber"}},
	{Name: "instance-of-check-skip-when-dispatch", Description: "Skip InstanceOfCheckForException when the `is ExceptionType` check is inside a `when (e) { is X -> ... }` dispatch on the caught variable. Kotlin's when-is dispatch is the idiomatic way to handle a group of related exception types with shared handlers.", Intent: "fp-reduction", TargetRules: []string{"InstanceOfCheckForException"}},
	{Name: "map-get-bang-skip-contains-key-filter", Description: "Skip MapGetWithNotNullAssertionOperator when the map[k]!! access is inside a .map { ... } lambda whose chain has a preceding .filter { map.containsKey(k) } — the filter guarantees the key is present.", Intent: "fp-reduction", TargetRules: []string{"MapGetWithNotNullAssertionOperator"}},
	{Name: "return-count-skip-when-initializer-guards", Description: "Skip ReturnCount for return statements inside a when-expression that's the initializer of a val/var binding — these are guard-branches in an assignment-style dispatch where some cases can't produce a value.", Intent: "fp-reduction", TargetRules: []string{"ReturnCount"}},
	{Name: "unsafe-call-skip-require-function-body", Description: "Skip UnsafeCallOnNullableType when the !! is the entire body of a single-expression function whose name starts with 'require' — the function name documents the precondition and the !! is the idiomatic implementation of that precondition.", Intent: "fp-reduction", TargetRules: []string{"UnsafeCallOnNullableType"}},
	{Name: "throws-count-exclude-guard-clauses", Description: "Enable guard-clause exclusion for ThrowsCount so leading if-check + throw preconditions don't count toward the function's throw total, matching ReturnCount's default behavior. Fixes the broken ExcludeGuardClauses code path that was never properly filtering.", Intent: "fp-reduction", TargetRules: []string{"ThrowsCount"}},
	{Name: "matching-declaration-name-skip-ext-fun-files", Description: "Skip MatchingDeclarationName when the file has top-level extension functions alongside a single class/object — the file is a 'type + extensions' group and its name reflects the theme, not any individual declaration.", Intent: "fp-reduction", TargetRules: []string{"MatchingDeclarationName"}},
}

type Set struct {
	enabled map[string]bool
}

func NewSet(names []string) Set {
	s := Set{enabled: make(map[string]bool, len(names))}
	for _, name := range names {
		if name == "" {
			continue
		}
		s.enabled[name] = true
	}
	return s
}

func (s Set) Enabled(name string) bool {
	return s.enabled != nil && s.enabled[name]
}

func (s Set) Names() []string {
	out := make([]string, 0, len(s.enabled))
	for name := range s.enabled {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

var (
	currentMu  sync.RWMutex
	currentSet = NewSet(nil)
)

func SetCurrent(names []string) {
	currentMu.Lock()
	defer currentMu.Unlock()
	currentSet = NewSet(names)
}

func Current() Set {
	currentMu.RLock()
	defer currentMu.RUnlock()
	return currentSet
}

func Enabled(name string) bool {
	currentMu.RLock()
	defer currentMu.RUnlock()
	return currentSet.Enabled(name)
}

func Definitions() []Definition {
	out := make([]Definition, len(knownDefinitions))
	copy(out, knownDefinitions)
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

func NamesForIntent(intent string) []string {
	intent = strings.TrimSpace(intent)
	if intent == "" {
		return nil
	}
	var out []string
	for _, def := range Definitions() {
		if def.Intent == intent {
			out = append(out, def.Name)
		}
	}
	sort.Strings(out)
	return out
}

func ParseCSV(input string) []string {
	return parseCSV(input, true)
}

func parseCSVOrdered(input string) []string {
	return parseCSV(input, false)
}

func parseCSV(input string, sortOutput bool) []string {
	if strings.TrimSpace(input) == "" {
		return nil
	}
	seen := make(map[string]bool)
	var out []string
	for _, raw := range strings.Split(input, ",") {
		name := strings.TrimSpace(raw)
		if name == "" || seen[name] {
			continue
		}
		seen[name] = true
		out = append(out, name)
	}
	if sortOutput {
		sort.Strings(out)
	}
	return out
}

func MergeEnabled(base []string, extra []string, disabled []string) []string {
	enabled := make(map[string]bool, len(base)+len(extra))
	for _, name := range base {
		enabled[name] = true
	}
	for _, name := range extra {
		enabled[name] = true
	}
	for _, name := range disabled {
		delete(enabled, name)
	}
	out := make([]string, 0, len(enabled))
	for name := range enabled {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

type MatrixCase struct {
	Name    string   `json:"name"`
	Enabled []string `json:"enabled"`
}

func BuildMatrix(spec string, candidates []string) ([]MatrixCase, error) {
	spec = strings.TrimSpace(spec)
	if spec == "" {
		return nil, fmt.Errorf("empty experiment matrix spec")
	}
	if len(candidates) == 0 {
		for _, def := range Definitions() {
			candidates = append(candidates, def.Name)
		}
	}
	candidates = ParseCSV(strings.Join(candidates, ","))
	if strings.Contains(spec, ";") {
		return parseExplicitCases(spec)
	}

	var cases []MatrixCase
	seen := make(map[string]bool)
	addCase := func(c MatrixCase) {
		key := c.Name + "|" + strings.Join(c.Enabled, ",")
		if seen[key] {
			return
		}
		seen[key] = true
		cases = append(cases, c)
	}

	for _, token := range parseCSVOrdered(spec) {
		switch token {
		case "baseline":
			addCase(MatrixCase{Name: "baseline"})
		case "singles", "all-singles":
			for _, name := range candidates {
				addCase(MatrixCase{Name: name, Enabled: []string{name}})
			}
		case "pairs":
			for i := 0; i < len(candidates); i++ {
				for j := i + 1; j < len(candidates); j++ {
					enabled := []string{candidates[i], candidates[j]}
					addCase(MatrixCase{Name: strings.Join(enabled, "+"), Enabled: enabled})
				}
			}
		case "cumulative":
			var enabled []string
			for _, name := range candidates {
				enabled = append(enabled, name)
				cp := append([]string(nil), enabled...)
				addCase(MatrixCase{Name: strings.Join(cp, "+"), Enabled: cp})
			}
		default:
			return nil, fmt.Errorf("unknown experiment matrix token %q", token)
		}
	}
	if len(cases) == 0 {
		return nil, fmt.Errorf("experiment matrix %q produced no cases", spec)
	}
	return cases, nil
}

func parseExplicitCases(spec string) ([]MatrixCase, error) {
	var cases []MatrixCase
	for _, raw := range strings.Split(spec, ";") {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		if raw == "baseline" {
			cases = append(cases, MatrixCase{Name: "baseline"})
			continue
		}
		var enabled []string
		for _, piece := range strings.Split(raw, "+") {
			name := strings.TrimSpace(piece)
			if name != "" {
				enabled = append(enabled, name)
			}
		}
		if len(enabled) == 0 {
			return nil, fmt.Errorf("invalid explicit experiment case %q", raw)
		}
		sort.Strings(enabled)
		cases = append(cases, MatrixCase{Name: strings.Join(enabled, "+"), Enabled: enabled})
	}
	if len(cases) == 0 {
		return nil, fmt.Errorf("experiment matrix %q produced no cases", spec)
	}
	return cases, nil
}
