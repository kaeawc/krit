package rules

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	v2 "github.com/kaeawc/krit/internal/rules/v2"
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
func (r *TranslatableMarkupMismatchRule) Confidence() float64 { return 0.8 }

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

func (r *TranslatableMarkupMismatchRule) check(ctx *v2.Context) {
	if ctx.ResourceIndex == nil {
		return
	}
	for _, resRoot := range resourceRootsFromIndex(ctx.ResourceIndex) {
		r.checkResourceRoot(ctx, resRoot)
	}
}

func (r *TranslatableMarkupMismatchRule) checkResourceRoot(ctx *v2.Context, resRoot string) {
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
	for _, v := range variants {
		records := map[string]stringMarkupRecord{}
		collectStringMarkup(v.dir, records)
		names := make([]string, 0, len(records))
		for n := range records {
			names = append(names, n)
		}
		sort.Strings(names)
		for _, name := range names {
			def, ok := defaults[name]
			if !ok {
				continue
			}
			vs := records[name]
			if !def.style.any() && !vs.style.any() {
				continue
			}
			if def.style.eq(vs.style) {
				continue
			}
			ctx.Emit(resourceFinding(vs.path, vs.line, r.BaseRule,
				fmt.Sprintf("String `%s` markup style differs from default: `values/` uses %s; `values-%s/` uses %s.",
					name, def.style.describe(), v.locale, vs.style.describe())))
		}
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
		if err == io.EOF {
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
		if strings.EqualFold(xmlLocalAttr(start.Attr, "translatable"), "false") {
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
