package rules

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/kaeawc/krit/internal/android"
	"github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// The four Lint issues of NetworkSecurityConfigDetector share one analysis of res/xml network-security-config files.
const (
	nscIssue     = "NetworkSecurityConfig"
	nscExpiry    = "PinSetExpiry"
	nscBackupPin = "MissingBackupPin"
	nscUserCerts = "AcceptsUserCertificates"
)

// NetworkSecurityConfigRule mirrors Lint's NetworkSecurityConfig: structural problems in a network-security-config file.
type NetworkSecurityConfigRule struct {
	ManifestBase
	AndroidRule
}

// PinSetExpiryRule mirrors Lint's PinSetExpiry: a pin-set that has expired or expires within ten days.
type PinSetExpiryRule struct {
	ManifestBase
	AndroidRule
}

// MissingBackupPinRule mirrors Lint's MissingBackupPin: a pin-set with a single pin.
type MissingBackupPinRule struct {
	ManifestBase
	AndroidRule
}

// AcceptsUserCertificatesRule mirrors Lint's AcceptsUserCertificates: base-config trust anchors that accept user certificates.
type AcceptsUserCertificatesRule struct {
	ManifestBase
	AndroidRule
}

// Confidence reports a high base confidence: the checks read the configuration file literally.
func (r *NetworkSecurityConfigRule) Confidence() float64 { return api.ConfidenceHigh }

// Confidence reports a high base confidence: the checks read the configuration file literally.
func (r *PinSetExpiryRule) Confidence() float64 { return api.ConfidenceHigh }

// Confidence reports a high base confidence: the checks read the configuration file literally.
func (r *MissingBackupPinRule) Confidence() float64 { return api.ConfidenceHigh }

// Confidence reports a high base confidence: the checks read the configuration file literally.
func (r *AcceptsUserCertificatesRule) Confidence() float64 { return api.ConfidenceHigh }

// nscToday returns the current date; tests replace it.
var nscToday = time.Now

type nscFinding struct {
	issue     string
	line, col int
	message   string
}

var (
	nscValidConfigTags = []string{"domain", "trust-anchors", "pin-set", "domain-config"}
	nscValidBaseTags   = []string{"domain-config", "base-config", "debug-overrides"}
	nscBase64Alphabet  = regexp.MustCompile(`^[A-Za-z0-9+/]*={0,2}$`)
)

type nscAnalyzer struct {
	data       []byte
	findings   []nscFinding
	debugNode  *android.XMLNode
	baseNode   *android.XMLNode
	lineStarts []int
}

func newNSCAnalyzer(data []byte) *nscAnalyzer {
	a := &nscAnalyzer{data: data, lineStarts: []int{0}}
	for i, b := range data {
		if b == '\n' {
			a.lineStarts = append(a.lineStarts, i+1)
		}
	}
	return a
}

func (a *nscAnalyzer) report(issue string, line, col int, msg string) {
	a.findings = append(a.findings, nscFinding{issue, line, col, msg})
}

func (a *nscAnalyzer) atName(issue string, n *android.XMLNode, msg string) {
	a.report(issue, n.Line, n.Col+1, msg)
}

func (a *nscAnalyzer) atNode(issue string, n *android.XMLNode, msg string) {
	a.report(issue, n.Line, n.Col, msg)
}

func nscAttrNode(n *android.XMLNode, name string) (android.XMLAttribute, bool) {
	for _, at := range n.Attrs {
		if at.Name == name {
			return at, true
		}
	}
	return android.XMLAttribute{}, false
}

// atValue reports at the start of an attribute value, after its opening quote.
func (a *nscAnalyzer) atValue(issue string, at android.XMLAttribute, msg string) {
	valueCol := at.Col + int(at.EndByte-at.StartByte) - len(at.Value) - 1
	a.report(issue, at.Line, valueCol, msg)
}

// firstText finds the first child text node of n: its raw value and the position of its first non-blank character.
func (a *nscAnalyzer) firstText(n *android.XMLNode) (text string, line, col int, isText bool) {
	i := int(n.StartByte)
	var quote byte
	for ; i < len(a.data); i++ {
		c := a.data[i]
		if quote != 0 {
			if c == quote {
				quote = 0
			}
			continue
		}
		if c == '"' || c == '\'' {
			quote = c
		} else if c == '>' {
			break
		}
	}
	if i >= len(a.data) || (i > 0 && a.data[i-1] == '/') {
		return "", 0, 0, false
	}
	start := i + 1
	end := start
	for end < len(a.data) && a.data[end] != '<' {
		end++
	}
	if end == start {
		return "", 0, 0, false
	}
	first := start
	for first < end && strings.ContainsRune(" \t\r\n", rune(a.data[first])) {
		first++
	}
	if first >= end {
		first = start
	}
	row := sort.Search(len(a.lineStarts), func(k int) bool { return a.lineStarts[k] > first }) - 1
	return string(a.data[start:end]), row + 1, first - a.lineStarts[row] + 1, true
}

func nscEditDistance(s, t string) int {
	if s == t {
		return 0
	}
	a, b := []rune(s), []rune(t)
	prev := make([]int, len(b)+1)
	cur := make([]int, len(b)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(a); i++ {
		cur[0] = i
		for j := 1; j <= len(b); j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			cur[j] = min(prev[j]+1, cur[j-1]+1, prev[j-1]+cost)
		}
		prev, cur = cur, prev
	}
	return prev[len(b)]
}

// nscSuggestions returns the valid names within edit distance 3 of name.
func nscSuggestions(name string, valid []string) []string {
	var out []string
	for _, v := range valid {
		d := len(name) - len(v)
		if d < 0 {
			d = -d
		}
		if d <= 3 && nscEditDistance(v, name) <= 3 {
			out = append(out, v)
		}
	}
	return out
}

func nscList(items []string) string {
	switch len(items) {
	case 1:
		return items[0]
	case 2:
		return items[0] + " or " + items[1]
	}
	return strings.Join(items[:len(items)-1], ", ") + " or " + items[len(items)-1]
}

func (a *nscAnalyzer) checkTagTypos(n *android.XMLNode, valid []string) bool {
	suggestions := nscSuggestions(n.Tag, valid)
	if len(suggestions) == 0 {
		return false
	}
	a.atName(nscIssue, n, fmt.Sprintf("Misspelled tag `<%s>`: Did you mean `%s`?", n.Tag, nscList(suggestions)))
	return true
}

func (a *nscAnalyzer) checkAttrTypos(n *android.XMLNode, attr string, required bool) {
	if _, ok := nscAttrNode(n, attr); ok {
		return
	}
	found := false
	var suggestions []string
	for _, at := range n.Attrs {
		if strings.TrimSpace(at.Name) != "" {
			suggestions = nscSuggestions(at.Name, []string{attr})
		}
		if len(suggestions) == 1 {
			a.atName(nscIssue, &android.XMLNode{Line: at.Line, Col: at.Col - 1}, fmt.Sprintf("Misspelled attribute `%s`: Did you mean `%s`?", at.Name, attr))
			found = true
		}
	}
	if !found && required {
		a.atName(nscIssue, n, fmt.Sprintf("Missing `%s` attribute", attr))
	}
}

func (a *nscAnalyzer) document(root *android.XMLNode) {
	if root == nil || root.Tag != "network-security-config" {
		return
	}
	seenDomains := map[string]*android.XMLNode{}
	for _, node := range root.Children {
		switch node.Tag {
		case "base-config":
			if a.baseNode != nil {
				a.reportExceeded("base-config", node, a.baseNode)
				continue
			}
			a.baseNode = node
			a.configElement(node, seenDomains)
			a.baseConfigElement(node)
		case "debug-overrides":
			if a.debugNode != nil {
				a.reportExceeded("debug-overrides", node, a.debugNode)
				continue
			}
			a.debugNode = node
			a.configElement(node, seenDomains)
		case "domain-config":
			a.configElement(node, seenDomains)
		default:
			if !a.checkTagTypos(node, nscValidBaseTags) {
				a.atName(nscIssue, node, fmt.Sprintf("Unexpected element `<%s>`", node.Tag))
			}
		}
	}
}

func (a *nscAnalyzer) reportExceeded(name string, node, _ *android.XMLNode) {
	a.atName(nscIssue, node, fmt.Sprintf("Expecting at most 1 `<%s>`", name))
}

func (a *nscAnalyzer) configElement(config *android.XMLNode, seenDomains map[string]*android.XMLNode) {
	isDomainConfig := config.Tag == "domain-config"
	const notAllowed = "`%s` element not allowed in `%s`"
	a.checkAttrTypos(config, "cleartextTrafficPermitted", false)
	var trustAnchors, pinSet *android.XMLNode
	for _, node := range config.Children {
		switch node.Tag {
		case "domain":
			if !isDomainConfig {
				a.atName(nscIssue, node, fmt.Sprintf(notAllowed, "domain", config.Tag))
				continue
			}
			a.checkAttrTypos(node, "includeSubdomains", true)
			name := strings.ToLower(strings.TrimSpace(node.Text))
			if _, dup := seenDomains[name]; dup {
				a.atFirstChild(node, "Duplicate domain names are not allowed")
			} else {
				seenDomains[name] = node
			}
		case "trust-anchors":
			if trustAnchors != nil {
				a.atName(nscIssue, node, "Multiple `<trust-anchors>` elements are not allowed")
				continue
			}
			trustAnchors = node
			a.trustAnchors(node)
		case "domain-config":
			if !isDomainConfig {
				a.atName(nscIssue, node, fmt.Sprintf("Nested `<domain-config>` elements are not allowed in `%s`", config.Tag))
				continue
			}
			a.configElement(node, seenDomains)
		case "pin-set":
			if !isDomainConfig {
				a.atName(nscIssue, node, fmt.Sprintf(notAllowed, "pin-set", config.Tag))
			}
			if pinSet != nil {
				a.atName(nscIssue, node, "Multiple `<pin-set>` elements are not allowed")
				continue
			}
			pinSet = node
			a.pinSet(node)
		default:
			a.checkTagTypos(node, nscValidConfigTags)
		}
	}
	if isDomainConfig && len(seenDomains) == 0 {
		a.atName(nscIssue, config, "No `<domain>` elements in `<domain-config>`")
	}
}

// atFirstChild reports at the first character of an element's text, as Lint does for a duplicated domain.
func (a *nscAnalyzer) atFirstChild(n *android.XMLNode, msg string) {
	if _, line, col, ok := a.firstText(n); ok {
		a.report(nscIssue, line, col, msg)
		return
	}
	a.atNode(nscIssue, n, msg)
}

func (a *nscAnalyzer) baseConfigElement(node *android.XMLNode) {
	for _, child := range node.Children {
		if child.Tag != "trust-anchors" {
			continue
		}
		for _, grand := range child.Children {
			if src, ok := nscAttrNode(grand, "src"); grand.Tag == "certificates" && ok && src.Value == "user" {
				a.atNode(nscUserCerts, grand, "The Network Security Configuration allows the use of user certificates in the release version of your app")
			}
		}
	}
}

var nscResourceURL = regexp.MustCompile(`^[@?]`)

func (a *nscAnalyzer) trustAnchors(node *android.XMLNode) {
	for _, child := range node.Children {
		if child.Tag != "certificates" {
			a.checkTagTypos(child, []string{"certificates"})
			continue
		}
		src, ok := nscAttrNode(child, "src")
		if !ok {
			a.checkAttrTypos(child, "src", true)
			continue
		}
		if !nscResourceURL.MatchString(src.Value) && src.Value != "user" && src.Value != "system" {
			a.atValue(nscIssue, src, "Unknown certificates `src` attribute. Expecting `system`, `user` or an @resource value")
		}
	}
}

func (a *nscAnalyzer) pinSet(node *android.XMLNode) {
	if exp, ok := nscAttrNode(node, "expiration"); ok {
		message := ""
		date, err := time.Parse("2006-01-02", exp.Value)
		if err != nil || len(exp.Value) != 10 {
			a.atValue(nscIssue, exp, "Invalid expiration in `pin-set`")
		} else {
			now := nscToday()
			today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
			switch {
			case date.Before(today):
				message = "`pin-set` has already expired"
			case date.Before(today.AddDate(0, 0, 10)):
				message = "`pin-set` is expiring soon"
			}
		}
		if message != "" {
			a.atValue(nscExpiry, exp, message)
		}
	} else {
		a.checkAttrTypos(node, "expiration", false)
	}
	pins, typos := 0, false
	for _, child := range node.Children {
		if child.Tag != "pin" {
			typos = a.checkTagTypos(child, []string{"pin"}) || typos
			continue
		}
		pins++
		if digest, ok := nscAttrNode(child, "digest"); ok {
			if !strings.EqualFold("SHA-256", digest.Value) {
				a.atValue(nscIssue, digest, "Invalid digest algorithm. Supported digests: `SHA-256`")
			}
		} else {
			a.checkAttrTypos(child, "digest", true)
		}
		text, line, col, isText := a.firstText(child)
		if !isText {
			a.atNode(nscIssue, child, "Missing pin digest")
			continue
		}
		decoded, ok := nscDecodeBase64(text)
		if !ok {
			a.report(nscIssue, line, col, "Invalid pin digest")
		} else if len(decoded) != 32 {
			a.report(nscIssue, line, col, fmt.Sprintf("Decoded digest length `%d` does not match expected length for `SHA-256` of `32`", len(decoded)))
		}
	}
	if typos {
		return
	}
	switch pins {
	case 0:
		a.atName(nscIssue, node, "Missing `<pin>` element(s)")
	case 1:
		a.atName(nscBackupPin, node, "A backup `<pin>` declaration is highly recommended")
	}
}

// nscDecodeBase64 decodes like java.util.Base64.getDecoder(): no whitespace, padding optional.
func nscDecodeBase64(s string) ([]byte, bool) {
	if !nscBase64Alphabet.MatchString(s) {
		return nil, false
	}
	if out, err := base64.StdEncoding.DecodeString(s); err == nil {
		return out, true
	}
	if out, err := base64.RawStdEncoding.DecodeString(strings.TrimRight(s, "=")); err == nil && !strings.Contains(s, "=") {
		return out, true
	}
	return nil, false
}

// nscConfigFiles lists the res/xml files next to a manifest; the main manifest also covers source sets without a manifest.
func nscConfigFiles(manifestPath string) []string {
	dir := filepath.Dir(manifestPath)
	roots := []string{dir}
	if filepath.Base(dir) == "main" && filepath.Base(filepath.Dir(dir)) == "src" {
		siblings, _ := os.ReadDir(filepath.Dir(dir))
		for _, e := range siblings {
			other := filepath.Join(filepath.Dir(dir), e.Name())
			if e.IsDir() && other != dir {
				if _, err := os.Stat(filepath.Join(other, "AndroidManifest.xml")); err != nil {
					roots = append(roots, other)
				}
			}
		}
	}
	var files []string
	for _, root := range roots {
		dirs, _ := filepath.Glob(filepath.Join(root, "res", "xml*"))
		for _, d := range dirs {
			entries, _ := os.ReadDir(d)
			for _, e := range entries {
				if !e.IsDir() && strings.HasSuffix(e.Name(), ".xml") {
					files = append(files, filepath.Join(d, e.Name()))
				}
			}
		}
	}
	sort.Strings(files)
	return files
}

func nscRun(ctx *api.Context, issue string) {
	m := ctx.Manifest
	if m == nil || m.Path == "" {
		return
	}
	for _, path := range nscConfigFiles(m.Path) {
		data, err := os.ReadFile(path)
		if err != nil || !strings.Contains(string(data), "network-security-config") {
			continue
		}
		root, err := android.ParseXMLAST(context.Background(), data)
		if err != nil {
			continue
		}
		a := newNSCAnalyzer(data)
		a.document(root)
		for _, f := range a.findings {
			if f.issue == issue {
				ctx.Emit(scanner.Finding{File: path, Line: f.line, Col: f.col, Message: f.message})
			}
		}
	}
}

func (r *NetworkSecurityConfigRule) check(ctx *api.Context)   { nscRun(ctx, nscIssue) }
func (r *PinSetExpiryRule) check(ctx *api.Context)            { nscRun(ctx, nscExpiry) }
func (r *MissingBackupPinRule) check(ctx *api.Context)        { nscRun(ctx, nscBackupPin) }
func (r *AcceptsUserCertificatesRule) check(ctx *api.Context) { nscRun(ctx, nscUserCerts) }
