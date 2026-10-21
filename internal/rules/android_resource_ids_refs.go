package rules

import (
	"fmt"
	"strings"

	"github.com/kaeawc/krit/internal/android"
	api "github.com/kaeawc/krit/internal/rules/api"
)

// ---------------------------------------------------------------------------
// IllegalResourceRefResource
// ---------------------------------------------------------------------------

// IllegalResourceRefResourceRule detects malformed resource references in XML
// attributes. Valid references must match @[+][type/]name or @android:type/name.
type IllegalResourceRefResourceRule struct {
	LayoutResourceBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence. Android resource-id rule. Detection flags R.id/R.layout usage patterns
// and naming conventions via structural checks on resources. Classified
// per roadmap/17.
func (r *IllegalResourceRefResourceRule) Confidence() float64 { return api.ConfidenceMedium }

func (r *IllegalResourceRefResourceRule) check(ctx *api.Context) {
	idx := ctx.ResourceIndex
	for _, layout := range idx.Layouts {
		walkViews(layout.RootView, func(v *android.View) {
			for attr, val := range v.Attributes {
				if !strings.HasPrefix(val, "@") {
					continue
				}
				// Skip tool and style references
				if strings.HasPrefix(attr, "tools:") {
					continue
				}
				// Skip @null
				if val == "@null" {
					continue
				}
				// Strip leading @[+]
				ref := val[1:]
				ref = strings.TrimPrefix(ref, "+")
				// Must contain a / separating type from name
				if !strings.Contains(ref, "/") {
					ctx.Emit(baseFinding(layout.FilePath, v.Line, r.BaseRule,
						fmt.Sprintf("Malformed resource reference `%s` in attribute `%s`. "+
							"Expected format `@[type]/name`.",
							val, attr)))
					continue
				}
				parts := strings.SplitN(ref, "/", 2)
				resType := parts[0]
				resName := parts[1]
				// Type part (possibly prefixed with "android:")
				resType = strings.TrimPrefix(resType, "android:")
				if resType == "" {
					ctx.Emit(baseFinding(layout.FilePath, v.Line, r.BaseRule,
						fmt.Sprintf("Malformed resource reference `%s` in attribute `%s`. "+
							"Missing resource type before `/`.",
							val, attr)))
					continue
				}
				if resName == "" {
					ctx.Emit(baseFinding(layout.FilePath, v.Line, r.BaseRule,
						fmt.Sprintf("Malformed resource reference `%s` in attribute `%s`. "+
							"Missing resource name after `/`.",
							val, attr)))
				}
			}
		})
	}
}

// ---------------------------------------------------------------------------
// WrongCaseResource
// ---------------------------------------------------------------------------

// WrongCaseResourceRule detects view tags with incorrect capitalization.
// Common mistakes include "Textview" instead of "TextView", "linearlayout"
// instead of "LinearLayout", etc.
type WrongCaseResourceRule struct {
	LayoutResourceBase
	AndroidRule
}

// wrongCaseFixes maps lowercase view names to their correct casing.
var wrongCaseFixes = map[string]string{
	"textview":                  "TextView",
	"imageview":                 "ImageView",
	"linearlayout":              "LinearLayout",
	"relativelayout":            "RelativeLayout",
	"framelayout":               "FrameLayout",
	"scrollview":                "ScrollView",
	"listview":                  "ListView",
	"gridview":                  "GridView",
	"webview":                   "WebView",
	"edittext":                  "EditText",
	"imagebutton":               "ImageButton",
	"checkbox":                  "CheckBox",
	"radiobutton":               "RadioButton",
	"radiogroup":                "RadioGroup",
	"switch":                    "Switch",
	"togglebutton":              "ToggleButton",
	"progressbar":               "ProgressBar",
	"seekbar":                   "SeekBar",
	"ratingbar":                 "RatingBar",
	"videoview":                 "VideoView",
	"viewflipper":               "ViewFlipper",
	"viewswitcher":              "ViewSwitcher",
	"horizontalscrollview":      "HorizontalScrollView",
	"tablelayout":               "TableLayout",
	"tablerow":                  "TableRow",
	"viewstub":                  "ViewStub",
	"surfaceview":               "SurfaceView",
	"textureview":               "TextureView",
	"constraintlayout":          "ConstraintLayout",
	"coordinatorlayout":         "CoordinatorLayout",
	"recyclerview":              "RecyclerView",
	"cardview":                  "CardView",
	"nestedscrollview":          "NestedScrollView",
	"autocompletetextview":      "AutoCompleteTextView",
	"multiautocompletetextview": "MultiAutoCompleteTextView",
}

// Confidence reports a tier-2 (medium) base confidence. Android resource-id rule. Detection flags R.id/R.layout usage patterns
// and naming conventions via structural checks on resources. Classified
// per roadmap/17.
func (r *WrongCaseResourceRule) Confidence() float64 { return api.ConfidenceMedium }

func (r *WrongCaseResourceRule) check(ctx *api.Context) {
	idx := ctx.ResourceIndex
	for _, layout := range idx.Layouts {
		walkViews(layout.RootView, func(v *android.View) {
			// Skip fully-qualified names (contain a dot)
			if strings.Contains(v.Type, ".") {
				return
			}
			lower := strings.ToLower(v.Type)
			correct, ok := wrongCaseFixes[lower]
			if !ok {
				return
			}
			if v.Type != correct {
				ctx.Emit(baseFinding(layout.FilePath, v.Line, r.BaseRule,
					fmt.Sprintf("View tag `%s` has wrong capitalization. Use `%s` instead.",
						v.Type, correct)))
			}
		})
	}
}

// ---------------------------------------------------------------------------
// WrongFolderResource
// ---------------------------------------------------------------------------

// WrongFolderResourceRule detects resources that reference drawables not found
// in any drawable or mipmap directory.
type WrongFolderResourceRule struct {
	LayoutResourceBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence. Android resource-id rule. Detection flags R.id/R.layout usage patterns
// and naming conventions via structural checks on resources. Classified
// per roadmap/17.
func (r *WrongFolderResourceRule) Confidence() float64 { return api.ConfidenceMedium }

func (r *WrongFolderResourceRule) check(ctx *api.Context) {
	idx := ctx.ResourceIndex
	if len(idx.Drawables) == 0 {
		return
	}
	drawableSet := make(map[string]bool, len(idx.Drawables))
	for _, d := range idx.Drawables {
		drawableSet[d] = true
	}
	for _, layout := range idx.Layouts {
		walkViews(layout.RootView, func(v *android.View) {
			for attr, val := range v.Attributes {
				if !strings.HasPrefix(val, "@drawable/") {
					continue
				}
				name := strings.TrimPrefix(val, "@drawable/")
				if name == "" {
					continue
				}
				if !drawableSet[name] {
					ctx.Emit(baseFinding(layout.FilePath, v.Line, r.BaseRule,
						fmt.Sprintf("Resource reference `%s` in attribute `%s` not found in any drawable or mipmap directory.",
							val, attr)))
				}
			}
		})
	}
}

// ---------------------------------------------------------------------------
// InvalidResourceFolderResource
// ---------------------------------------------------------------------------

// InvalidResourceFolderResourceRule checks layout file paths for invalid
// resource folder prefixes. Valid prefixes are: layout, drawable, mipmap,
// values, raw, xml, anim, animator, color, menu, font, navigation, transition.
type InvalidResourceFolderResourceRule struct {
	LayoutResourceBase
	AndroidRule
}

// validResFolderPrefixes lists the valid Android resource folder name prefixes.
var validResFolderPrefixes = []string{
	"layout", "drawable", "mipmap", "values", "raw", "xml",
	"anim", "animator", "color", "menu", "font", "navigation", "transition",
}

func isValidResFolderName(folder string) bool {
	for _, prefix := range validResFolderPrefixes {
		if folder == prefix || strings.HasPrefix(folder, prefix+"-") {
			return true
		}
	}
	return false
}

// Confidence reports a tier-2 (medium) base confidence. Android resource-id rule. Detection flags R.id/R.layout usage patterns
// and naming conventions via structural checks on resources. Classified
// per roadmap/17.
func (r *InvalidResourceFolderResourceRule) Confidence() float64 { return api.ConfidenceMedium }

func (r *InvalidResourceFolderResourceRule) check(ctx *api.Context) {
	idx := ctx.ResourceIndex
	for _, layout := range idx.Layouts {
		path := layout.FilePath
		// Extract the folder name from the path: look for res/<folder>/file.xml
		resIdx := strings.Index(path, "res/")
		if resIdx < 0 {
			resIdx = strings.Index(path, "res\\")
		}
		if resIdx < 0 {
			continue
		}
		afterRes := path[resIdx+4:]
		slashIdx := strings.IndexAny(afterRes, "/\\")
		if slashIdx < 0 {
			continue
		}
		folder := afterRes[:slashIdx]
		if !isValidResFolderName(folder) {
			ctx.Emit(baseFinding(layout.FilePath, 1, r.BaseRule,
				fmt.Sprintf("Invalid resource folder name `%s`. "+
					"Expected one of: layout, drawable, mipmap, values, raw, xml, anim, animator, color, menu, font, navigation, transition.",
					folder)))
		}
	}
}
