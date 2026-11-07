package rules

import (
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	api "github.com/kaeawc/krit/internal/rules/api"
)

// TranslatableMarkupMismatchRule flags <string> resources whose markup
// style differs across locale variants — e.g. the default uses HTML
// markup like <b>bold</b> while a translated variant uses Markdown
// (**bold**) or plain text (or vice versa).
type TranslatableMarkupMismatchRule struct {
	ValuesStringsResourceBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence. Markup
// detection is regex-based; literal `**` or `<b ` lookalikes in
// translator text can occasionally produce false positives.
func (r *TranslatableMarkupMismatchRule) Confidence() float64 { return api.ConfidenceMediumHigh }

type markupStyle struct {
	html bool
	md   bool
}

func (s markupStyle) any() bool             { return s.html || s.md }
func (s markupStyle) eq(o markupStyle) bool { return s.html == o.html && s.md == o.md }
func (s markupStyle) describe() string {
	switch {
	case s.html && s.md:
		return "HTML and Markdown markup"
	case s.html:
		return "HTML markup (e.g. `<b>`)"
	case s.md:
		return "Markdown markup (e.g. `**bold**`)"
	default:
		return "plain text (no markup)"
	}
}

type stringMarkupRecord struct {
	path  string
	line  int
	style markupStyle
}

var htmlInlineTags = map[string]bool{
	"b": true, "i": true, "u": true, "em": true, "strong": true,
	"big": true, "small": true, "sub": true, "sup": true,
	"tt": true, "font": true, "br": true,
}

var (
	htmlMarkupTagRE = regexp.MustCompile(`(?i)<\s*/?\s*(` + htmlInlineTagAlternation() + `)\b`)
	markdownBoldRE  = regexp.MustCompile(`\*\*[^*\n]+\*\*|__[^_\n]+__`)
)

func htmlInlineTagAlternation() string {
	tags := make([]string, 0, len(htmlInlineTags))
	for t := range htmlInlineTags {
		tags = append(tags, t)
	}
	sort.Strings(tags)
	return strings.Join(tags, "|")
}

func (r *TranslatableMarkupMismatchRule) check(ctx *api.Context) {
	if ctx.ResourceIndex == nil {
		return
	}
	for _, resRoot := range resourceRootsFromIndex(ctx.ResourceIndex) {
		r.checkResourceRoot(ctx, resRoot)
	}
}

func (r *TranslatableMarkupMismatchRule) checkResourceRoot(ctx *api.Context, resRoot string) {
	entries, err := os.ReadDir(resRoot)
	if err != nil {
		return
	}
	defaults := map[string]stringMarkupRecord{}
	type variantDir struct {
		locale string
		dir    string
	}
	var variants []variantDir
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		if name == "values" {
			collectStringMarkup(filepath.Join(resRoot, name), defaults)
			continue
		}
		locale, ok := localeTagFromValuesDir(name)
		if !ok {
			continue
		}
		variants = append(variants, variantDir{locale: locale, dir: filepath.Join(resRoot, name)})
	}
	sort.Slice(variants, func(i, j int) bool { return variants[i].locale < variants[j].locale })
	variantRecords := make([]map[string]stringMarkupRecord, len(variants))
	// translationsStripped marks default markup that no translation keeps and every translation drops to
	// plain text; that pattern comes from a translation pipeline that strips tags, not from one mistake.
	translationsStripped := map[string]bool{}
	for i, v := range variants {
		variantRecords[i] = map[string]stringMarkupRecord{}
		collectStringMarkup(v.dir, variantRecords[i])
	}
	for name, def := range defaults {
		if !def.style.any() {
			continue
		}
		stripped, seen := true, false
		for _, records := range variantRecords {
			if vs, ok := records[name]; ok {
				seen = true
				if vs.style.any() {
					stripped = false
				}
			}
		}
		translationsStripped[name] = seen && stripped
	}
	// Like the other translation-consistency checks, report each string key once, on the first
	// mismatching locale, and count the remaining locales that differ.
	type mismatch struct {
		rec    stringMarkupRecord
		locale string
	}
	byName := map[string][]mismatch{}
	for i, v := range variants {
		records := variantRecords[i]
		names := make([]string, 0, len(records))
		for n := range records {
			names = append(names, n)
		}
		sort.Strings(names)
		for _, name := range names {
			def, ok := defaults[name]
			if !ok || translationsStripped[name] {
				continue
			}
			vs := records[name]
			if !def.style.any() && !vs.style.md {
				// A plain default is only a baseline for literal Markdown leaking in; extra HTML tags in a translation are not lost markup.
				continue
			}
			if def.style.eq(vs.style) {
				continue
			}
			byName[name] = append(byName[name], mismatch{rec: vs, locale: filepath.Base(v.dir)})
		}
	}
	keys := make([]string, 0, len(byName))
	for name := range byName {
		keys = append(keys, name)
	}
	sort.Strings(keys)
	for _, name := range keys {
		all := byName[name]
		first := all[0]
		msg := fmt.Sprintf("String `%s` markup style differs from default: `values/` uses %s; `%s/` uses %s.",
			name, defaults[name].style.describe(), first.locale, first.rec.style.describe())
		switch len(all) {
		case 1:
		case 2:
			msg += " 1 other locale differs too."
		default:
			msg += fmt.Sprintf(" %d other locales differ too.", len(all)-1)
		}
		ctx.Emit(baseFinding(first.rec.path, first.rec.line, r.BaseRule, msg))
	}
}

func collectStringMarkup(dir string, out map[string]stringMarkupRecord) {
	files, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	names := make([]string, 0, len(files))
	for _, f := range files {
		if f.IsDir() {
			continue
		}
		lower := strings.ToLower(f.Name())
		if !strings.HasPrefix(lower, "strings") || !strings.HasSuffix(lower, ".xml") {
			continue
		}
		names = append(names, f.Name())
	}
	sort.Strings(names)
	for _, fname := range names {
		path := filepath.Join(dir, fname)
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		parseStringsXMLForMarkup(path, data, out)
	}
}

func parseStringsXMLForMarkup(path string, data []byte, out map[string]stringMarkupRecord) {
	dec := xml.NewDecoder(bytes.NewReader(data))
	for {
		tok, err := dec.Token()
		if errors.Is(err, io.EOF) {
			return
		}
		if err != nil {
			return
		}
		start, ok := tok.(xml.StartElement)
		if !ok {
			continue
		}
		if start.Name.Local != "string" {
			continue
		}
		name := xmlLocalAttr(start.Attr, "name")
		if name == "" {
			_ = skipXMLBody(dec)
			continue
		}
		if strings.EqualFold(xmlLocalAttr(start.Attr, "translatable"), "false") ||
			strings.EqualFold(xmlLocalAttr(start.Attr, "formatted"), "false") {
			_ = skipXMLBody(dec)
			continue
		}
		line := lineAtByteOffset(data, dec.InputOffset())
		style, err := readStringMarkupStyle(dec)
		if err != nil {
			return
		}
		if _, exists := out[name]; exists {
			continue
		}
		out[name] = stringMarkupRecord{path: path, line: line, style: style}
	}
}

func readStringMarkupStyle(dec *xml.Decoder) (markupStyle, error) {
	var s markupStyle
	var sb strings.Builder
	depth := 1
	for depth > 0 {
		tok, err := dec.Token()
		if err != nil {
			return s, err
		}
		switch t := tok.(type) {
		case xml.CharData:
			sb.Write([]byte(t))
		case xml.StartElement:
			if htmlInlineTags[strings.ToLower(t.Name.Local)] {
				s.html = true
			}
			depth++
		case xml.EndElement:
			depth--
		}
	}
	text := sb.String()
	if !s.html && htmlMarkupTagRE.MatchString(text) {
		s.html = true
	}
	if markdownBoldRE.MatchString(text) {
		s.md = true
	}
	return s, nil
}

func skipXMLBody(dec *xml.Decoder) error {
	depth := 1
	for depth > 0 {
		tok, err := dec.Token()
		if err != nil {
			return err
		}
		switch tok.(type) {
		case xml.StartElement:
			depth++
		case xml.EndElement:
			depth--
		}
	}
	return nil
}

func xmlLocalAttr(attrs []xml.Attr, name string) string {
	for _, a := range attrs {
		if a.Name.Local == name {
			return a.Value
		}
	}
	return ""
}

func lineAtByteOffset(data []byte, offset int64) int {
	if offset < 0 {
		offset = 0
	}
	if int(offset) > len(data) {
		offset = int64(len(data))
	}
	return 1 + bytes.Count(data[:int(offset)], []byte{'\n'})
}
