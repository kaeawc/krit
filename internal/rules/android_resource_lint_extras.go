package rules

// Resource checks ported from Lint that need little context: ChildInNonViewGroup and EnforceUTF8.

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/kaeawc/krit/internal/android"
	api "github.com/kaeawc/krit/internal/rules/api"
)

// ChildInNonViewGroupResourceRule reports XML children of framework views that are not ViewGroups.
type ChildInNonViewGroupResourceRule struct {
	LayoutResourceBase
	AndroidRule
}

// Confidence reports a tier-1 (high) base confidence: only known framework tags are judged.
func (r *ChildInNonViewGroupResourceRule) Confidence() float64 { return api.ConfidenceHigh }

// frameworkNonViewGroups are the framework views Lint's SDK data knows are not ViewGroups. Unknown tags, including
// custom views, are treated as possible ViewGroups.
var frameworkNonViewGroups = map[string]bool{
	"SurfaceView": true, "TextureView": true, "View": true, "ViewStub": true, "AbsSeekBar": true, "AnalogClock": true,
	"AutoCompleteTextView": true, "Button": true, "CheckBox": true, "CheckedTextView": true, "Chronometer": true,
	"CompoundButton": true, "DigitalClock": true, "EditText": true, "ImageButton": true, "ImageView": true,
	"MultiAutoCompleteTextView": true, "ProgressBar": true, "QuickContactBadge": true, "RadioButton": true, "RatingBar": true,
	"SeekBar": true, "Space": true, "Switch": true, "TextClock": true, "TextView": true, "ToggleButton": true,
	"VideoView": true, "ZoomButton": true,
}

// frameworkViewSimpleName strips the package of an android.widget, android.view or android.webkit tag.
func frameworkViewSimpleName(tag string) string {
	for _, prefix := range []string{"android.widget.", "android.view.", "android.webkit."} {
		if strings.HasPrefix(tag, prefix) {
			return tag[len(prefix):]
		}
	}
	return tag
}

func (r *ChildInNonViewGroupResourceRule) check(ctx *api.Context) {
	for _, layout := range ctx.ResourceIndex.AllLayouts() {
		walkViewsWithParentUnlessIgnored(layout.RootView, nil, []string{"ChildInNonViewGroup"}, func(v, parent *android.View) {
			if parent == nil || isLayoutMarkerTag(parent.Type) || isLayoutMarkerTag(v.Type) {
				return
			}
			if frameworkNonViewGroups[frameworkViewSimpleName(parent.Type)] {
				ctx.Emit(baseFindingAt(layout.FilePath, v.Line, v.Col+1, r.BaseRule, "A "+parent.Type+" should have no children declared in XML"))
			}
		})
	}
}

// EnforceUtf8ResourceRule reports resource XML files that declare an encoding other than UTF-8.
type EnforceUtf8ResourceRule struct {
	LayoutResourceBase
	AndroidRule
}

// Confidence reports a tier-1 (high) base confidence: it reads the XML declaration.
func (r *EnforceUtf8ResourceRule) Confidence() float64 { return api.ConfidenceHigh }

var xmlEncodingPattern = regexp.MustCompile(`^\s*<\?xml[^>]*?\bencoding\s*=\s*["']([^"']*)["']`)

func (r *EnforceUtf8ResourceRule) check(ctx *api.Context) {
	idx := ctx.ResourceIndex
	for _, root := range resourceRootsFromIndex(idx) {
		dirs, err := os.ReadDir(root)
		if err != nil {
			continue
		}
		var files []string
		for _, dir := range dirs {
			if !dir.IsDir() {
				continue
			}
			entries, err := os.ReadDir(filepath.Join(root, dir.Name()))
			if err != nil {
				continue
			}
			for _, e := range entries {
				if !e.IsDir() && strings.HasSuffix(e.Name(), ".xml") {
					files = append(files, filepath.Join(root, dir.Name(), e.Name()))
				}
			}
		}
		sort.Strings(files)
		for _, path := range files {
			head, err := readFileHead(path, 512)
			if err != nil {
				continue
			}
			m := xmlEncodingPattern.FindSubmatchIndex(head)
			if m == nil {
				continue
			}
			encoding := string(head[m[2]:m[3]])
			// Lint accepts UTF-8 and UTF-16 declarations; "utf8" without the dash is reported.
			if lower := strings.ToLower(encoding); lower == "utf-8" || strings.HasPrefix(lower, "utf-16") {
				continue
			}
			// The first line holds the declaration, so the value column is its byte offset plus one.
			ctx.Emit(baseFindingAt(path, 1, m[2]+1, r.BaseRule, fmt.Sprintf(
				"%s: Not using UTF-8 as the file encoding. This can lead to subtle bugs with non-ascii characters", encoding)))
		}
	}
}

func readFileHead(path string, n int) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	buf := make([]byte, n)
	read, err := f.Read(buf)
	if err != nil && read == 0 {
		return nil, err
	}
	return buf[:read], nil
}
