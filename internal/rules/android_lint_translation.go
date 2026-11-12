package rules

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"
	"sync"

	"github.com/kaeawc/krit/internal/android"
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// This file ports the resource-folder checks of Lint's TranslationDetector (MissingTranslation, ExtraTranslation,
// MissingDefaultResource, Untranslatable) and DuplicateResourceDetector (DuplicateDefinition). Every check works on
// one module: the res directories of its source sets together.

// Lint issue IDs of the ported checks.
const (
	lintMissingTranslation = "MissingTranslation"
	lintExtraTranslation   = "ExtraTranslation"
	lintMissingDefault     = "MissingDefaultResource"
	lintUntranslatable     = "Untranslatable"
	lintDuplicateDef       = "DuplicateDefinition"
)

// translationFinding is one report of the translation checks.
type translationFinding struct {
	issue string
	file  string
	line  int
	col   int
	msg   string
}

// translationRes is one resource declared by a resource file or element.
type translationRes struct {
	typ      string // Lint resource type name: string, plurals, array, color, layout, ...
	name     string
	tag      string // XML tag for values resources
	folder   string // folder name, such as values-es
	folderTy string // folder type, such as values
	file     string
	node     *android.XMLNode // nil for whole-file resources
	lang     string           // language of the folder, "" when it has none
	isLocale bool
	isDef    bool
	tools    string // tools:locale of the document root
	order    int
}

// serviceKeyNames are the string names SdkUtils.isServiceKey accepts, plus the two Maps keys Lint also skips.
var serviceKeyNames = map[string]bool{
	"gcm_defaultSenderId": true, "google_app_id": true, "google_api_key": true, "google_storage_bucket": true,
	"ga_trackingId": true, "default_web_client_id": true, "firebase_database_url": true,
	"google_crash_reporting_api_key": true, "google_maps_key": true, "google_maps_key_instructions": true,
}

var (
	translationDensityRe = regexp.MustCompile(`^(?:ldpi|mdpi|hdpi|xhdpi|xxhdpi|xxxhdpi|tvdpi|anydpi|nodpi|\d+dpi|v\d+)$`)
	translationMccRe     = regexp.MustCompile(`^mnc?\d+$`)
)

// translationFolderInfo classifies a res subfolder: its type, whether Lint reads it, whether its qualifiers are only
// density or version ("default"), and its locale language.
func translationFolderInfo(folder string) (folderTy string, valid, isDefault bool, lang string) {
	folderTy, quals, hasQuals := strings.Cut(folder, "-")
	if !hasQuals {
		return folderTy, true, true, ""
	}
	if loc, ok := parseFolderLocale(folder); ok {
		return folderTy, true, false, loc.language
	}
	isDefault = true
	for _, tok := range strings.Split(quals, "-") {
		switch {
		case translationDensityRe.MatchString(tok):
		case folderOtherQualifierRe.MatchString(tok) || translationMccRe.MatchString(tok):
			isDefault = false
		default:
			return folderTy, false, false, ""
		}
	}
	return folderTy, true, isDefault, ""
}

// valuesResourceType maps a child element of <resources> to the Lint resource type, or "" when Lint has none or
// skips it (id, public, aapt).
func valuesResourceType(n *android.XMLNode) string {
	switch n.Tag {
	case "string", "plurals", "color", "dimen", "style", "bool", "integer", "fraction", "drawable", "attr":
		return n.Tag
	case "string-array", "integer-array", "array":
		return "array"
	case "declare-styleable":
		return "styleable"
	case "item":
		switch t := n.Attr("type"); t {
		case "id", "":
			return ""
		case "string-array", "integer-array", "array":
			return "array"
		default:
			return t
		}
	}
	return ""
}

// translationFieldName is ResourcesUtil.resourceNameToFieldName.
func translationFieldName(name string) string {
	return strings.NewReplacer(".", "_", "-", "_", ":", "_").Replace(name)
}

// fileResourceName is SdkUtils.fileNameToResourceName.
func fileResourceName(file string) string {
	if strings.HasSuffix(file, ".9.png") && len(file) > len(".9.png") {
		return file[:len(file)-len(".9.png")]
	}
	if dot := strings.LastIndexByte(file, '.'); dot > 0 {
		return file[:dot]
	}
	return file
}

// javaStringHash is String.hashCode for ASCII text.
func javaStringHash(s string) int32 {
	var h int32
	for i := 0; i < len(s); i++ {
		h = 31*h + int32(s[i])
	}
	return h
}

// javaHashSetOrder sorts values the way iterating a java.util.HashSet filled in this order would.
func javaHashSetOrder(values []string) []string {
	capacity := 16
	for len(values)*4 > capacity*3 {
		capacity *= 2
	}
	bucket := func(s string) int {
		h := uint32(javaStringHash(s))
		return int((h ^ h>>16) & uint32(capacity-1))
	}
	out := append([]string(nil), values...)
	sort.SliceStable(out, func(i, j int) bool { return bucket(out[i]) < bucket(out[j]) })
	return out
}

// translationModule is the resource data of one module.
type translationModule struct {
	res          []*translationRes
	locales      []string // distinct languages of values folders, in discovery order
	localeSeen   map[string]bool
	nonTranslate map[string]bool
	findings     []translationFinding
}

// moduleResRoots groups res directories by module: <module>/src/<set>/res share one module.
func moduleResRoots(roots []string) map[string][]string {
	out := map[string][]string{}
	for _, root := range roots {
		setDir := filepath.Dir(root)
		module := setDir
		if filepath.Base(filepath.Dir(setDir)) == "src" {
			module = filepath.Dir(filepath.Dir(setDir))
		}
		out[module] = append(out[module], root)
	}
	return out
}

// analyzeTranslations runs all five checks over the res roots of every module.
func analyzeTranslations(roots []string) []translationFinding {
	modules := moduleResRoots(roots)
	keys := make([]string, 0, len(modules))
	for k := range modules {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var out []translationFinding
	for _, k := range keys {
		rs := modules[k]
		sort.Strings(rs)
		m := &translationModule{localeSeen: map[string]bool{}, nonTranslate: map[string]bool{}}
		for _, root := range rs {
			m.readRoot(root)
		}
		m.checkDuplicates()
		m.checkTranslations()
		out = append(out, m.findings...)
	}
	return out
}

// readRoot reads every folder and file under one res directory.
func (m *translationModule) readRoot(root string) {
	folders, err := os.ReadDir(root)
	if err != nil {
		return
	}
	for _, fe := range folders {
		if !fe.IsDir() {
			continue
		}
		folder := fe.Name()
		folderTy, valid, isDef, lang := translationFolderInfo(folder)
		if !valid {
			continue
		}
		if folderTy == "values" && lang != "" && !m.localeSeen[lang] {
			m.localeSeen[lang] = true
			m.locales = append(m.locales, lang)
		}
		files, _ := os.ReadDir(filepath.Join(root, folder))
		for _, f := range files {
			if f.IsDir() || strings.HasPrefix(f.Name(), ".") {
				continue
			}
			path := filepath.Join(root, folder, f.Name())
			if folderTy == "values" {
				if strings.HasSuffix(f.Name(), ".xml") {
					m.readValuesFile(path, folder, isDef, lang)
				}
				continue
			}
			m.res = append(m.res, &translationRes{typ: folderTy, name: fileResourceName(f.Name()), folder: folder, folderTy: folderTy,
				file: path, lang: lang, isLocale: lang != "", isDef: isDef, order: len(m.res)})
		}
	}
}

// translationIgnoredFile is TranslationDetector.ignoreFile.
func translationIgnoredFile(name string) bool {
	return strings.HasPrefix(name, "donottranslate") || name == "analytics.xml"
}

// readValuesFile records the resources of one values XML file.
func (m *translationModule) readValuesFile(path, folder string, isDef bool, lang string) {
	if translationIgnoredFile(filepath.Base(path)) {
		return
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	root, err := android.ParseXMLAST(context.Background(), data)
	if err != nil || root == nil || root.Tag != "resources" {
		return
	}
	if root.Attr("translatable") == "false" {
		if folder != "values" && lang != "" {
			a := xmlAttr(root, "translatable")
			m.report(lintUntranslatable, path, a.Line, a.Col,
				fmt.Sprintf("This resource folder is marked as non-translatable yet is in a translated resource folder (%s)", lintLanguageDescription(lang)))
		}
		return
	}
	tools := root.Attr("tools:locale")
	if tools != "" && lang != "" && lang != strings.SplitN(tools, "-", 2)[0] {
		a := xmlAttr(root, "tools:locale")
		m.report(lintMissingTranslation, path, a.Line, a.Col+len(a.Name)+2,
			fmt.Sprintf("Suspicious `tools:locale` declaration of language `%s`; the parent folder `%s` implies language %s", tools, folder, lang))
	}
	for _, child := range root.Children {
		typ := valuesResourceType(child)
		if typ == "" {
			continue
		}
		if !hasAttr(child, "name") {
			m.report(lintMissingTranslation, path, child.Line, child.Col, fmt.Sprintf("Missing `name` attribute in `<%s>` declaration", child.Tag))
			continue
		}
		if strings.TrimSpace(child.Attr("name")) == "" {
			continue
		}
		if child.Tag == "style" || child.Tag == "declare-styleable" {
			m.checkChildNames(path, child)
		}
		m.res = append(m.res, &translationRes{typ: typ, name: translationFieldName(child.Attr("name")), tag: child.Tag, folder: folder,
			folderTy: "values", file: path, node: child, lang: lang, isLocale: lang != "", isDef: isDef, tools: tools, order: len(m.res)})
	}
}

// checkChildNames reports repeated child names of a style or declare-styleable (DuplicateResourceDetector.checkUniqueNames).
func (m *translationModule) checkChildNames(path string, parent *android.XMLNode) {
	seen := map[string]bool{}
	for _, c := range parent.Children {
		if !hasAttr(c, "name") {
			continue
		}
		name := c.Attr("name")
		if seen[name] {
			a := xmlAttr(c, "name")
			m.report(lintDuplicateDef, path, a.Line, a.Col, fmt.Sprintf("`%s` has already been defined in this `<%s>`", name, parent.Tag))
		}
		seen[name] = true
	}
}

func hasAttr(n *android.XMLNode, name string) bool { return xmlAttr(n, name).Name != "" }

func xmlAttr(n *android.XMLNode, name string) android.XMLAttribute {
	for _, a := range n.Attrs {
		if a.Name == name {
			return a
		}
	}
	return android.XMLAttribute{}
}

func (m *translationModule) report(issue, file string, line, col int, msg string) {
	m.findings = append(m.findings, translationFinding{issue: issue, file: file, line: max(line, 1), col: max(col, 1), msg: msg})
}

// reportAtName reports at the name attribute of a values element, or the file start otherwise.
func (m *translationModule) reportAtName(issue string, r *translationRes, msg string) {
	if r.node == nil {
		line, col := rootElementPos(r.file)
		m.report(issue, r.file, line, col, msg)
		return
	}
	a := xmlAttr(r.node, "name")
	m.report(issue, r.file, a.Line, a.Col, msg)
}

// rootElementPos is the 1-based position of the first element of an XML file, or 1:1 for other files.
func rootElementPos(path string) (line, col int) {
	if !strings.HasSuffix(path, ".xml") {
		return 1, 1
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return 1, 1
	}
	line, col = 1, 1
	for i := 0; i < len(data); i++ {
		switch {
		case data[i] == '\n':
			line, col = line+1, 1
			continue
		case data[i] == '<' && i+1 < len(data) && data[i+1] != '?' && data[i+1] != '!':
			return line, col
		case data[i] == '<' && bytes.HasPrefix(data[i:], []byte("<!--")):
			end := bytes.Index(data[i:], []byte("-->"))
			if end < 0 {
				return 1, 1
			}
			for _, c := range data[i : i+end+3] {
				if c == '\n' {
					line, col = line+1, 1
				} else {
					col++
				}
			}
			i += end + 2
			continue
		}
		col++
	}
	return 1, 1
}

// checkDuplicates is DuplicateResourceDetector: a name declared twice for one type in one values folder.
func (m *translationModule) checkDuplicates() {
	seen := map[string]bool{}
	for _, r := range m.res {
		if r.node == nil || hasAttr(r.node, "product") {
			continue
		}
		key := filepath.Dir(r.file) + "\x00" + r.typ + "\x00" + r.name
		if seen[key] {
			m.reportAtName(lintDuplicateDef, r, fmt.Sprintf("`%s` has already been defined in this folder", r.name))
		}
		seen[key] = true
	}
}

// translatable is handleNonTranslatable, minus its reports.
func nonTranslatable(r *translationRes) bool {
	if r.node != nil && hasAttr(r.node, "translatable") && !strings.EqualFold(r.node.Attr("translatable"), "true") {
		return true
	}
	return serviceKeyNames[r.name]
}

func isTranslatableType(typ string) bool { return typ == "string" || typ == "plurals" }

// checkTranslations is TranslationDetector's two passes.
func (m *translationModule) checkTranslations() {
	defaults := map[string]map[string]bool{}
	translations := map[string]map[string]bool{}
	for _, r := range m.res {
		if r.isDef {
			if defaults[r.typ] == nil {
				defaults[r.typ] = map[string]bool{}
			}
			defaults[r.typ][r.name] = true
		}
		if r.node != nil && isTranslatableType(r.typ) && nonTranslatable(r) {
			m.nonTranslate[r.name] = true
		}
	}
	for _, r := range m.res {
		if r.node == nil || !isTranslatableType(r.typ) {
			continue
		}
		if nt := m.untranslatable(r); nt {
			continue
		}
		lang := r.lang
		if lang == "" && r.tools != "" {
			lang = strings.SplitN(r.tools, "-", 2)[0]
		}
		if r.isDef && lang == "" {
			continue
		}
		if translations[r.name] == nil {
			translations[r.name] = map[string]bool{}
		}
		translations[r.name][lang] = true
	}
	order := javaHashSetOrder(m.locales)
	for _, r := range m.res {
		if r.isDef {
			m.checkMissing(r, order, translations)
			continue
		}
		if !defaults[r.typ][r.name] {
			m.reportExtra(r)
		}
	}
}

// untranslatable is handleNonTranslatable for a string: it reports translations of untranslatable strings and
// returns whether the string is untranslatable.
func (m *translationModule) untranslatable(r *translationRes) bool {
	locale := !r.isDef && r.lang != ""
	if hasAttr(r.node, "translatable") && !strings.EqualFold(r.node.Attr("translatable"), "true") {
		if locale {
			m.reportUntranslatable(r, "translatable", fmt.Sprintf("The resource string \"%s\" is marked as translatable=\"false\", but is translated to %s here",
				r.name, lintLanguageDescription(r.lang)))
		}
		return true
	}
	if serviceKeyNames[r.name] {
		return true
	}
	if locale && m.nonTranslate[r.name] {
		m.reportUntranslatable(r, "name", fmt.Sprintf("The resource string \"%s\" has been marked as translatable=\"false\" elsewhere (usually in the `values` folder), but is translated to %s here",
			r.name, lintLanguageDescription(r.lang)))
	}
	return false
}

func (m *translationModule) reportUntranslatable(r *translationRes, attr, msg string) {
	a := xmlAttr(r.node, attr)
	m.report(lintUntranslatable, r.file, a.Line, a.Col, msg)
}

// checkMissing reports a default string or plural that some locale does not translate.
func (m *translationModule) checkMissing(r *translationRes, locales []string, translations map[string]map[string]bool) {
	if r.node == nil || !isTranslatableType(r.typ) || nonTranslatable(r) {
		return
	}
	var missing []string
	for _, l := range locales {
		if !translations[r.name][l] {
			missing = append(missing, lintLanguageDescription(l))
		}
	}
	if len(missing) == 0 {
		return
	}
	sep := ", "
	if len(missing) == 2 {
		sep = " or "
	}
	m.reportAtName(lintMissingTranslation, r, fmt.Sprintf("\"%s\" is not translated in %s", r.name, strings.Join(missing, sep)))
}

// reportExtra reports a resource of a qualified folder that has no default declaration.
func (m *translationModule) reportExtra(r *translationRes) {
	issue := lintMissingDefault
	if r.typ == "string" || r.typ == "array" && r.tag == "string-array" {
		issue = lintExtraTranslation
	}
	var msg string
	switch r.typ {
	case "string":
		msg = fmt.Sprintf("\"%s\" is translated here but not found in default locale", r.name)
	case "drawable":
		msg = fmt.Sprintf("The drawable \"%s\" in %s has no declaration in the base `drawable` folder or in a `drawable-`*density*`dpi` folder; this can lead to crashes when the drawable is queried in a configuration that does not match this qualifier", r.name, r.folder)
	default:
		msg = fmt.Sprintf("The %s \"%s\" in %s has no declaration in the base `%s` folder; this can lead to crashes when the resource is queried in a configuration that does not match this qualifier", r.typ, r.name, r.folder, r.folderTy)
	}
	m.reportAtName(issue, r, msg)
}

// lintLanguageDescription is TranslationDetector.getLanguageDescription: "fr" (French).
func lintLanguageDescription(lang string) string {
	if name, ok := lintLanguageNames[lang]; ok {
		return fmt.Sprintf("%q (%s)", lang, name)
	}
	return fmt.Sprintf("%q", lang)
}

// translationRoots lists the res directories of the module set the rule is scanning.
func translationRoots(ctx *api.Context) []string {
	if ctx.ResourceIndex == nil {
		return nil
	}
	seen := map[string]bool{}
	var roots []string
	add := func(dir string) {
		if !seen[dir] {
			seen[dir] = true
			roots = append(roots, dir)
		}
	}
	for _, dir := range ctx.ResourceIndex.ResDirs {
		add(dir)
		// The index only lists src/main/res; the other non-test source sets of the module merge with it.
		for _, sib := range android.SiblingSourceSetResDirs(dir) {
			add(sib)
		}
	}
	sort.Strings(roots)
	return roots
}

// translationMemo holds the analysis of one resource index so the five translation rules share it.
type translationMemo struct {
	idx      *android.ResourceIndex
	roots    []string
	once     sync.Once
	findings []translationFinding
}

var (
	translationMemoMu   sync.Mutex
	translationMemoLast *translationMemo
)

// translationFindingsFor runs analyzeTranslations once per resource index and roots set.
func translationFindingsFor(idx *android.ResourceIndex, roots []string) []translationFinding {
	translationMemoMu.Lock()
	m := translationMemoLast
	if m == nil || m.idx != idx || !slices.Equal(m.roots, roots) {
		m = &translationMemo{idx: idx, roots: roots}
		translationMemoLast = m
	}
	translationMemoMu.Unlock()
	m.once.Do(func() { m.findings = analyzeTranslations(roots) })
	return m.findings
}

// emitTranslationFindings emits the findings of one issue.
func emitTranslationFindings(ctx *api.Context, rule BaseRule, issue string) {
	roots := translationRoots(ctx)
	if len(roots) == 0 {
		return
	}
	for _, f := range translationFindingsFor(ctx.ResourceIndex, roots) {
		if f.issue == issue {
			ctx.Emit(scanner.Finding{File: f.file, Line: f.line, Col: f.col, RuleSet: rule.RuleSetName, Rule: rule.RuleName, Severity: rule.Sev, Message: f.msg})
		}
	}
}

// MissingTranslationRule mirrors Lint's MissingTranslation: a default string or plural that some locale lacks.
type MissingTranslationRule struct {
	ResourceBase
	AndroidRule
}

// ExtraTranslationRule mirrors Lint's ExtraTranslation: a translated string with no default declaration.
type ExtraTranslationRule struct {
	ResourceBase
	AndroidRule
}

// MissingDefaultResourceRule mirrors Lint's MissingDefaultResource: a qualified resource with no base declaration.
type MissingDefaultResourceRule struct {
	ResourceBase
	AndroidRule
}

// UntranslatableRule mirrors Lint's Untranslatable: a translation of a string marked translatable="false".
type UntranslatableRule struct {
	ResourceBase
	AndroidRule
}

// DuplicateDefinitionRule mirrors Lint's DuplicateDefinition: one resource name declared twice in a values folder.
type DuplicateDefinitionRule struct {
	ResourceBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence.
func (r *MissingTranslationRule) Confidence() float64 { return api.ConfidenceMedium }

// Confidence reports a tier-2 (medium) base confidence.
func (r *ExtraTranslationRule) Confidence() float64 { return api.ConfidenceMedium }

// Confidence reports a tier-2 (medium) base confidence.
func (r *MissingDefaultResourceRule) Confidence() float64 { return api.ConfidenceMedium }

// Confidence reports a tier-2 (medium) base confidence.
func (r *UntranslatableRule) Confidence() float64 { return api.ConfidenceMedium }

// Confidence reports a tier-2 (medium) base confidence.
func (r *DuplicateDefinitionRule) Confidence() float64 { return api.ConfidenceMedium }

func (r *MissingTranslationRule) check(ctx *api.Context) {
	emitTranslationFindings(ctx, r.BaseRule, lintMissingTranslation)
}

func (r *ExtraTranslationRule) check(ctx *api.Context) {
	emitTranslationFindings(ctx, r.BaseRule, lintExtraTranslation)
}

func (r *MissingDefaultResourceRule) check(ctx *api.Context) {
	emitTranslationFindings(ctx, r.BaseRule, lintMissingDefault)
}

func (r *UntranslatableRule) check(ctx *api.Context) {
	emitTranslationFindings(ctx, r.BaseRule, lintUntranslatable)
}

func (r *DuplicateDefinitionRule) check(ctx *api.Context) {
	emitTranslationFindings(ctx, r.BaseRule, lintDuplicateDef)
}

func registerTranslationRules() {
	reg := func(r AndroidRule, impl any, conf float64, check func(*api.Context)) {
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: api.Severity(r.Sev),
			Needs: api.NeedsResources, AndroidDeps: uint32(AndroidDepValues), Confidence: conf, Implementation: impl, Check: check,
		})
	}
	mt := &MissingTranslationRule{AndroidRule: lintAndroidRule(lintMissingTranslation, "Incomplete translation", ALCMessages, ALSError, 8)}
	reg(mt.AndroidRule, mt, mt.Confidence(), mt.check)
	et := &ExtraTranslationRule{AndroidRule: lintAndroidRule(lintExtraTranslation, "Extra translation", ALCMessages, ALSFatal, 6)}
	reg(et.AndroidRule, et, et.Confidence(), et.check)
	md := &MissingDefaultResourceRule{AndroidRule: lintAndroidRule(lintMissingDefault, "Missing Default", ALCCorrectness, ALSFatal, 6)}
	reg(md.AndroidRule, md, md.Confidence(), md.check)
	ut := &UntranslatableRule{AndroidRule: lintAndroidRule(lintUntranslatable, "Translated Untranslatable", ALCMessages, ALSWarning, 6)}
	reg(ut.AndroidRule, ut, ut.Confidence(), ut.check)
	dd := &DuplicateDefinitionRule{AndroidRule: lintAndroidRule(lintDuplicateDef, "Duplicate definitions of resources", ALCCorrectness, ALSError, 6)}
	reg(dd.AndroidRule, dd, dd.Confidence(), dd.check)
}
