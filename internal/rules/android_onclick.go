package rules

// OnClick rule — flags android:onClick handlers in layout XML that have no
// matching `fun name(view: View)` method on the Activity/Fragment that
// inflates the layout. The handler is resolved by reflection at runtime, so
// a missing or wrong-signature method compiles cleanly but throws
// NoSuchMethodException when the user taps the view.
//
// Cross-references the resource index (layout XML attributes) with the
// project's parsed Kotlin files. A class is considered to inflate a layout
// if it calls `setContentView(R.layout.X)` or `inflate(R.layout.X, ...)`.

import (
	"fmt"

	"github.com/kaeawc/krit/internal/android"
	v2 "github.com/kaeawc/krit/internal/rules/v2"
	"github.com/kaeawc/krit/internal/scanner"
)

type onClickHandlerSite struct {
	method     string
	layoutName string
	layoutPath string
	line       int
	viewType   string
}

type onClickClassSummary struct {
	file     *scanner.File
	classIdx uint32
	name     string
	layouts  map[string]struct{}
	methods  map[string]onClickMethodInfo
}

type onClickMethodInfo struct {
	idx          uint32
	isPrivate    bool
	paramCount   int
	hasViewParam bool
}

func (r *OnClickRule) check(ctx *v2.Context) {
	if ctx == nil || ctx.ResourceIndex == nil || len(ctx.ParsedFiles) == 0 {
		return
	}
	handlersByLayout := collectOnClickHandlerSites(ctx.ResourceIndex)
	if len(handlersByLayout) == 0 {
		return
	}
	for _, file := range ctx.ParsedFiles {
		if file == nil || file.FlatTree == nil {
			continue
		}
		for idx := range file.FlatTree.Nodes {
			classIdx := uint32(idx)
			if file.FlatType(classIdx) != "class_declaration" {
				continue
			}
			summary := summarizeOnClickClass(file, classIdx)
			if summary == nil || len(summary.layouts) == 0 {
				continue
			}
			emitOnClickFindings(ctx, r.BaseRule, summary, handlersByLayout)
		}
	}
}

func collectOnClickHandlerSites(idx *android.ResourceIndex) map[string][]onClickHandlerSite {
	out := make(map[string][]onClickHandlerSite)
	add := func(layout *android.Layout) {
		if layout == nil {
			return
		}
		walkViews(layout.RootView, func(v *android.View) {
			handler := v.Attributes["android:onClick"]
			if handler == "" || !isValidKotlinIdentifier(handler) {
				return
			}
			out[layout.Name] = append(out[layout.Name], onClickHandlerSite{
				method:     handler,
				layoutName: layout.Name,
				layoutPath: layout.FilePath,
				line:       v.Line,
				viewType:   v.Type,
			})
		})
	}
	for _, layout := range idx.Layouts {
		add(layout)
	}
	return out
}

func summarizeOnClickClass(file *scanner.File, classIdx uint32) *onClickClassSummary {
	body, _ := file.FlatFindChild(classIdx, "class_body")
	if body == 0 {
		return nil
	}
	summary := &onClickClassSummary{
		file:     file,
		classIdx: classIdx,
		name:     extractIdentifierFlat(file, classIdx),
		layouts:  make(map[string]struct{}),
		methods:  make(map[string]onClickMethodInfo),
	}
	collectClassMethods(file, body, summary.methods)
	collectClassInflatedLayouts(file, classIdx, summary.layouts)
	return summary
}

func collectClassMethods(file *scanner.File, body uint32, out map[string]onClickMethodInfo) {
	for child := file.FlatFirstChild(body); child != 0; child = file.FlatNextSib(child) {
		if file.FlatType(child) != "function_declaration" {
			continue
		}
		nameIdx, _ := file.FlatFindChild(child, "simple_identifier")
		if nameIdx == 0 {
			continue
		}
		name := file.FlatNodeText(nameIdx)
		if name == "" {
			continue
		}
		paramCount, hasView := analyzeFunctionViewSignature(file, child)
		out[name] = onClickMethodInfo{
			idx:          child,
			isPrivate:    file.FlatHasModifier(child, "private"),
			paramCount:   paramCount,
			hasViewParam: hasView,
		}
	}
}

func analyzeFunctionViewSignature(file *scanner.File, funcDecl uint32) (paramCount int, hasViewParam bool) {
	params, _ := file.FlatFindChild(funcDecl, "function_value_parameters")
	if params == 0 {
		return 0, false
	}
	for child := file.FlatFirstChild(params); child != 0; child = file.FlatNextSib(child) {
		if file.FlatType(child) != "parameter" {
			continue
		}
		paramCount++
		if paramCount == 1 && parameterTypeNameIsView(file, child) {
			hasViewParam = true
		}
	}
	return paramCount, hasViewParam
}

func parameterTypeNameIsView(file *scanner.File, parameter uint32) bool {
	userType, _ := file.FlatFindChild(parameter, "user_type")
	if userType == 0 {
		return false
	}
	ident := flatLastChildOfType(file, userType, "type_identifier")
	if ident == 0 {
		return false
	}
	return file.FlatNodeText(ident) == "View"
}

// collectClassInflatedLayouts walks the class subtree once, capturing the
// layout names referenced by setContentView(R.layout.X) and inflate(R.layout.X, ...).
func collectClassInflatedLayouts(file *scanner.File, classIdx uint32, out map[string]struct{}) {
	classNode := file.FlatTree.Nodes[classIdx]
	end := classNode.EndByte
	for cursor := classIdx + 1; int(cursor) < len(file.FlatTree.Nodes); cursor++ {
		node := file.FlatTree.Nodes[cursor]
		if node.StartByte >= end {
			break
		}
		if file.FlatType(cursor) != "call_expression" {
			continue
		}
		name := flatCallExpressionName(file, cursor)
		if name != "setContentView" && name != "inflate" {
			continue
		}
		if layout, ok := layoutInflationLayoutName(file, cursor); ok {
			out[layout] = struct{}{}
		}
	}
}

func emitOnClickFindings(ctx *v2.Context, rule BaseRule, summary *onClickClassSummary, handlersByLayout map[string][]onClickHandlerSite) {
	for layoutName := range summary.layouts {
		for _, site := range handlersByLayout[layoutName] {
			method, ok := summary.methods[site.method]
			if !ok {
				ctx.Emit(resourceFinding(site.layoutPath, site.line, rule,
					fmt.Sprintf("`android:onClick=\"%s\"` on `%s` has no matching method on `%s`. Declare `fun %s(view: View)` or remove the attribute.",
						site.method, site.viewType, classDisplayName(summary), site.method)))
				continue
			}
			if method.isPrivate || method.paramCount != 1 || !method.hasViewParam {
				ctx.Emit(resourceFinding(site.layoutPath, site.line, rule,
					fmt.Sprintf("`android:onClick=\"%s\"` on `%s` requires a public method `fun %s(view: View)` on `%s`. The current signature does not match and will throw NoSuchMethodException at runtime.",
						site.method, site.viewType, site.method, classDisplayName(summary))))
			}
		}
	}
}

func classDisplayName(summary *onClickClassSummary) string {
	if summary == nil || summary.name == "" {
		return "<class>"
	}
	return summary.name
}

