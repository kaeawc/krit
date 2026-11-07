package rules

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/kaeawc/krit/internal/scanner"

	api "github.com/kaeawc/krit/internal/rules/api"
)

// EasterEggRule ports Android Lint's EasterEgg (CommentDetector): a comment that
// contains the unicode escape for a comment end, written */. Java
// translates the escape before it parses comments, so the comment ends there and
// the text after it is compiled as code. The check covers Java line and block
// comments (not javadoc, as in Lint) and the comments of the XML files under res/ and
// the manifest of each source set that holds parsed code. Kotlin does not translate unicode escapes in comments, so Kotlin
// files are not checked.
//
// The krit-only check that used this name is HiddenFeatureCommentRule.
type EasterEggRule struct{ AndroidRule }

// Confidence reports a tier-1 (high) base confidence: the escape is matched exactly inside comments.
func (r *EasterEggRule) Confidence() float64 { return api.ConfidenceHigh }

const easterEggMessage = "Code might be hidden here; found unicode escape sequence which is interpreted as comment end, compiled code follows"

// commentEndEscapeRe matches */ ignoring case, as Lint does.
var commentEndEscapeRe = regexp.MustCompile(`(?i)\\u002a\\u002f`)

// xmlCommentRe matches an XML comment, across lines.
var xmlCommentRe = regexp.MustCompile(`(?s)<!--.*?-->`)

func (r *EasterEggRule) check(ctx *api.Context) {
	for _, file := range ctx.ParsedFiles {
		if file == nil || file.FlatTree == nil || file.Language != scanner.LangJava {
			continue
		}
		for _, kind := range []string{"line_comment", "block_comment"} {
			file.FlatWalkNodes(0, kind, func(node uint32) {
				text := file.FlatNodeText(node)
				// A javadoc comment is not checked.
				if strings.HasPrefix(text, "/**") {
					return
				}
				for _, loc := range commentEndEscapeRe.FindAllStringIndex(text, -1) {
					line, col := commentOffsetPosition(file, node, loc[0])
					ctx.Emit(r.Finding(file, line, col, easterEggMessage))
				}
			})
		}
	}
	seen := map[string]bool{}
	for _, sourceSet := range easterEggSourceSets(ctx.ParsedFiles) {
		r.checkXMLFile(ctx, filepath.Join(sourceSet, "AndroidManifest.xml"), seen)
		dirs, err := os.ReadDir(filepath.Join(sourceSet, "res"))
		if err != nil {
			continue
		}
		for _, dir := range dirs {
			if !dir.IsDir() {
				continue
			}
			resDir := filepath.Join(sourceSet, "res", dir.Name())
			files, err := os.ReadDir(resDir)
			if err != nil {
				continue
			}
			for _, f := range files {
				if !f.IsDir() && strings.HasSuffix(f.Name(), ".xml") {
					r.checkXMLFile(ctx, filepath.Join(resDir, f.Name()), seen)
				}
			}
		}
	}
}

// easterEggSourceSets returns the Gradle source-set directories (.../src/main and the like) that hold the parsed
// files, in sorted order. The manifest and res/ folder of a source set sit beside its code.
func easterEggSourceSets(files []*scanner.File) []string {
	seen := map[string]bool{}
	var sets []string
	for _, file := range files {
		if file == nil {
			continue
		}
		parts := strings.Split(filepath.ToSlash(file.Path), "/")
		for i := len(parts) - 2; i > 0; i-- {
			if parts[i-1] == "src" {
				dir := filepath.FromSlash(strings.Join(parts[:i+1], "/"))
				if !seen[dir] {
					seen[dir] = true
					sets = append(sets, dir)
				}
				break
			}
		}
	}
	sort.Strings(sets)
	return sets
}

func (r *EasterEggRule) checkXMLFile(ctx *api.Context, path string, seen map[string]bool) {
	if seen[path] {
		return
	}
	seen[path] = true
	data, err := os.ReadFile(path)
	if err != nil || !commentEndEscapeRe.Match(data) {
		return
	}
	text := string(data)
	for _, comment := range xmlCommentRe.FindAllStringIndex(text, -1) {
		// Lint reports an XML comment's escape on the line the comment starts on.
		line, col := textOffsetPosition(text, comment[0])
		for range commentEndEscapeRe.FindAllStringIndex(text[comment[0]:comment[1]], -1) {
			ctx.Emit(baseFindingAt(path, line, col, r.BaseRule, easterEggMessage))
		}
	}
}

// textOffsetPosition returns the 1-based line and column of byte offset within text.
func textOffsetPosition(text string, offset int) (int, int) {
	line, col := 1, 1
	for i := 0; i < offset && i < len(text); i++ {
		if text[i] == '\n' {
			line++
			col = 1
		} else {
			col++
		}
	}
	return line, col
}
