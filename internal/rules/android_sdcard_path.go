package rules

import (
	"strings"

	"github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

const (
	sdCardPathMessage = "Do not hardcode \"/sdcard/\"; use `Environment.getExternalStorageDirectory().getPath()` instead"
	dataPathMessage   = "Do not hardcode \"/data/\"; use `Context.getFilesDir().getPath()` instead"
)

// sdCardPathMessageFor mirrors Lint's SdCardDetector: it checks the start of each literal string value.
func sdCardPathMessageFor(value string) string {
	switch {
	case strings.HasPrefix(value, "/sdcard"), strings.HasPrefix(value, "/mnt/sdcard/"),
		strings.HasPrefix(value, "/system/media/sdcard"), strings.HasPrefix(value, "file://sdcard/"),
		strings.HasPrefix(value, "file:///sdcard/"):
		return sdCardPathMessage
	case strings.HasPrefix(value, "/data/data/"), strings.HasPrefix(value, "/data/user/"):
		return dataPathMessage
	}
	return ""
}

// checkSdCardPath reports a Java literal, or each literal fragment of a Kotlin string, that starts with a hardcoded
// storage path. Lint splits a Kotlin string at each escape and template entry and reports the fragment start.
func checkSdCardPath(ctx *api.Context) {
	file, idx := ctx.File, ctx.Idx
	if file.Language != scanner.LangKotlin {
		if msg := sdCardPathMessageFor(literalStringValue(file, idx)); msg != "" {
			ctx.EmitAt(file.FlatRow(idx)+1, file.FlatCol(idx)+1, msg)
		}
		return
	}
	raw := strings.HasPrefix(file.FlatNodeText(idx), `"""`)
	var pieces [][2]int
	for child := file.FlatFirstChild(idx); child != 0; child = file.FlatNextSib(child) {
		switch file.FlatType(child) {
		case "string_content":
			start := int(file.FlatStartByte(child))
			if raw {
				pieces = append(pieces, [2]int{start, int(file.FlatEndByte(child))})
			} else {
				pieces = append(pieces, kotlinEscapeFreeRuns(string(file.Content[start:file.FlatEndByte(child)]), start)...)
			}
		case "\"", "\"\"\"":
		default:
			pieces = append(pieces, [2]int{-1, -1})
		}
	}
	for _, piece := range pieces {
		if piece[0] < 0 {
			continue
		}
		msg := sdCardPathMessageFor(string(file.Content[piece[0]:piece[1]]))
		if msg == "" {
			continue
		}
		ctx.Emit(lintFindingAtOffset(file, piece[0], piece[1], "", msg))
	}
}

// kotlinEscapeFreeRuns splits the content of a Kotlin string at its backslash escapes and returns the byte ranges
// (offset by base) of the runs between them.
func kotlinEscapeFreeRuns(text string, base int) [][2]int {
	var runs [][2]int
	runStart := 0
	for i := 0; i < len(text); {
		if text[i] != '\\' {
			i++
			continue
		}
		if i > runStart {
			runs = append(runs, [2]int{base + runStart, base + i})
		}
		width := 2
		if i+1 < len(text) && text[i+1] == 'u' {
			width = 6
		}
		i += width
		runStart = min(i, len(text))
	}
	if runStart < len(text) {
		runs = append(runs, [2]int{base + runStart, base + len(text)})
	}
	return runs
}

// literalStringValue returns the Java string literal's value start: empty when it begins with an escape sequence,
// and without the indentation line of a text block.
func literalStringValue(file *scanner.File, idx uint32) string {
	for child := file.FlatFirstChild(idx); child != 0; child = file.FlatNextSib(child) {
		if file.FlatType(child) == "escape_sequence" {
			return ""
		}
		if file.FlatType(child) == "string_fragment" {
			break
		}
	}
	content := stringLiteralContent(file, idx)
	if strings.HasPrefix(file.FlatNodeText(idx), `"""`) {
		if _, rest, ok := strings.Cut(content, "\n"); ok {
			return strings.TrimLeft(rest, " \t")
		}
	}
	return content
}
