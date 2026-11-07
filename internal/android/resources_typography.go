package android

import (
	"bytes"
	"encoding/xml"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/kaeawc/krit/internal/scanner"
)

// serviceKeyStrings are the string names Android tooling generates for Google services; Lint skips them.
var serviceKeyStrings = map[string]bool{
	"gcm_defaultSenderId": true, "google_app_id": true, "google_api_key": true, "google_storage_bucket": true,
	"ga_trackingID": true, "default_web_client_id": true, "firebase_database_url": true, "google_crash_reporting_api_key": true,
}

// typographyFrame is one open element during scanTypographyTexts.
type typographyFrame struct {
	name   string
	attrs  []xml.Attr
	record bool
}

// scanTypographyTexts records the text and CDATA nodes of translatable <string> elements and of the <item> children
// of <string-array> and <plurals>, each at the position of its first non-blank character.
func (idx *ResourceIndex) scanTypographyTexts(path string, data []byte) {
	dec := xml.NewDecoder(bytes.NewReader(data))
	file := &scanner.File{Content: data, Language: scanner.LangXML, Path: path}
	var stack []typographyFrame
	prev := int64(0)
	for {
		tok, err := dec.Token()
		end := dec.InputOffset()
		if err != nil {
			return
		}
		start := prev
		prev = end
		switch t := tok.(type) {
		case xml.StartElement:
			stack = append(stack, typographyStartFrame(stack, t))
		case xml.EndElement:
			if len(stack) > 0 {
				stack = stack[:len(stack)-1]
			}
		case xml.CharData:
			idx.recordTypographyText(path, data, file, stack, t, start, end)
		}
	}
}

// typographyStartFrame builds the frame for a start element nested under stack.
func typographyStartFrame(stack []typographyFrame, t xml.StartElement) typographyFrame {
	f := typographyFrame{name: t.Name.Local, attrs: t.Attr}
	switch {
	case len(stack) == 1 && (f.name == "string" || f.name == "string-array" || f.name == "plurals"):
		f.record = typographyElementRecorded(t.Attr)
	case len(stack) == 2 && f.name == "item" && stack[1].record && stack[1].name != "string":
		f.record = true
	}
	return f
}

// recordTypographyText appends the character data spanning data[start:end] when its element is recorded.
func (idx *ResourceIndex) recordTypographyText(path string, data []byte, file *scanner.File, stack []typographyFrame, t xml.CharData, start, end int64) {
	if len(stack) == 0 {
		return
	}
	top := stack[len(stack)-1]
	owner := top
	if top.name == "item" && len(stack) >= 2 {
		owner = stack[len(stack)-2]
	}
	isString := top.name == "string" && len(stack) == 2
	isItem := top.name == "item" && len(stack) == 3
	if !top.record || (!isString && !isItem) {
		return
	}
	raw := data[start:end]
	cdata := bytes.HasPrefix(raw, []byte("<![CDATA["))
	// Lint skips CDATA that holds markup, since its tags are not text.
	if cdata && strings.Contains(string(t), "<") {
		return
	}
	if cdata {
		raw = raw[len("<![CDATA["):]
		start += int64(len("<![CDATA["))
	}
	offset := start + int64(len(raw)-len(bytes.TrimLeftFunc(raw, unicode.IsSpace)))
	line := file.RowForByte(int(offset)) + 1
	lineStart := bytes.LastIndexByte(data[:offset], '\n') + 1
	idx.TypographyTexts = append(idx.TypographyTexts, TypographyText{
		FilePath: path,
		Name:     xmlAttr(owner.attrs, "name"),
		Text:     string(t),
		CData:    cdata,
		Line:     line,
		Col:      utf8.RuneCount(data[lineStart:offset]) + 1,
		Offset:   int(offset),
	})
}

// typographyElementRecorded applies Lint's skips: service keys and strings that are not translatable.
func typographyElementRecorded(attrs []xml.Attr) bool {
	if serviceKeyStrings[xmlAttr(attrs, "name")] {
		return false
	}
	if v := xmlAttr(attrs, "translatable"); v != "" && v != "true" {
		return false
	}
	return true
}
