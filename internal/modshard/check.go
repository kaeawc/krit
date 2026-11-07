package modshard

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

// checkContext is the parsed FIR check request, just enough to key and merge per-module checks.
type checkContext struct {
	ID          int64
	files       map[string]bool
	rules       []string
	ruleConfigs json.RawMessage
	testFiles   map[string]bool
	scanPaths   map[string]string
	sdkLevels   map[string]json.RawMessage
}

// parseCheckContext returns nil when there is no usable request.
func parseCheckContext(data []byte) *checkContext {
	if len(data) == 0 {
		return nil
	}
	var raw struct {
		ID          int64                      `json:"id"`
		Files       []struct{ Path string }    `json:"files"`
		Rules       []string                   `json:"rules"`
		RuleConfigs json.RawMessage            `json:"ruleConfigs"`
		TestFiles   []string                   `json:"testFiles"`
		ScanPaths   map[string]string          `json:"scanPaths"`
		SDKLevels   map[string]json.RawMessage `json:"sdkLevels"`
	}
	if err := json.Unmarshal(data, &raw); err != nil || len(raw.Rules) == 0 || len(raw.Files) == 0 {
		return nil
	}
	c := &checkContext{ID: raw.ID, rules: append([]string(nil), raw.Rules...), ruleConfigs: raw.RuleConfigs,
		files: map[string]bool{}, testFiles: map[string]bool{}, scanPaths: raw.ScanPaths, sdkLevels: raw.SDKLevels}
	sort.Strings(c.rules)
	for _, f := range raw.Files {
		c.files[f.Path] = true
	}
	for _, f := range raw.TestFiles {
		c.testFiles[f] = true
	}
	return c
}

// requestedAmong returns the requested files that are among files, sorted. Nil-safe.
func (c *checkContext) requestedAmong(files []string) []string {
	if c == nil {
		return nil
	}
	var own []string
	for _, f := range files {
		if c.files[f] {
			own = append(own, f)
		}
	}
	sort.Strings(own)
	return own
}

// contextFor hashes everything the check verdicts of own depend on besides the module's sources.
func (c *checkContext) contextFor(own []string) string {
	h := sha256.New()
	fmt.Fprintf(h, "%q|%s|", c.rules, c.ruleConfigs)
	for _, f := range own {
		sdk, _ := json.Marshal(c.sdkLevels[f])
		fmt.Fprintf(h, "%s|%t|%s|%s;", f, c.testFiles[f], c.scanPaths[f], sdk)
	}
	return hex.EncodeToString(h.Sum(nil))[:16]
}

// checkPart is one module's check response; Findings stay raw so the schema is not repeated here.
type checkPart struct {
	Succeeded  int                          `json:"succeeded"`
	Skipped    int                          `json:"skipped"`
	Findings   []json.RawMessage            `json:"findings"`
	Crashed    map[string]string            `json:"crashed"`
	Rules      []string                     `json:"rules"`
	ErrorFiles map[string]string            `json:"errorFiles"`
	RuleErrors map[string]map[string]string `json:"ruleErrors"`
	Warning    string                       `json:"warning,omitempty"`
}

// mergeChecks combines the units' check responses into one response for the whole request. A
// requested file no module checked is reported as not authoritative, so the caller keeps its own
// findings for it instead of treating it as clean.
func mergeChecks(in Input, units []Unit, keys []string, requested map[string][]string, c *checkContext) ([]byte, error) {
	out := checkPart{Findings: []json.RawMessage{}, Crashed: map[string]string{}, Rules: []string{},
		ErrorFiles: map[string]string{}, RuleErrors: map[string]map[string]string{}}
	rules := map[string]bool{}
	checked := map[string]bool{}
	for i, u := range units {
		own := requested[u.Spec.ID]
		if len(own) == 0 {
			continue
		}
		data, err := os.ReadFile(filepath.Join(in.CacheDir, keys[i]+".check.json"))
		if err != nil {
			continue
		}
		var part checkPart
		if err := json.Unmarshal(data, &part); err != nil {
			continue
		}
		for _, f := range own {
			checked[f] = true
		}
		out.Succeeded += part.Succeeded
		out.Skipped += part.Skipped
		out.Findings = append(out.Findings, part.Findings...)
		for k, v := range part.Crashed {
			out.Crashed[k] = v
		}
		for _, r := range part.Rules {
			rules[r] = true
		}
		for k, v := range part.ErrorFiles {
			out.ErrorFiles[k] = v
		}
		for rule, files := range part.RuleErrors {
			if out.RuleErrors[rule] == nil {
				out.RuleErrors[rule] = map[string]string{}
			}
			for f, e := range files {
				out.RuleErrors[rule][f] = e
			}
		}
		if out.Warning == "" {
			out.Warning = part.Warning
		}
	}
	for f := range c.files {
		if !checked[f] {
			if _, already := out.ErrorFiles[f]; !already {
				out.ErrorFiles[f] = "krit-fir: not checked; no module analysis covered the file"
			}
			out.Skipped++
		}
	}
	for r := range rules {
		out.Rules = append(out.Rules, r)
	}
	sort.Strings(out.Rules)
	resp := struct {
		ID int64 `json:"id"`
		checkPart
	}{ID: c.ID, checkPart: out}
	return json.Marshal(resp)
}
