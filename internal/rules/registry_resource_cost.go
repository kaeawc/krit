package rules

import (
	"bytes"
	"fmt"
	v2 "github.com/kaeawc/krit/internal/rules/v2"
	"strconv"
	"strings"
)

func registerResourceCostRules() {

	// --- from resource_cost.go ---
	{
		r := &BufferedReadWithoutBufferRule{BaseRule: BaseRule{RuleName: "BufferedReadWithoutBuffer", RuleSetName: "resource-cost", Sev: "info", Desc: "Detects FileInputStream.read() without BufferedInputStream wrapping for efficient reads."}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"call_expression"}, Confidence: 0.75, OriginalV1: r,
			Check: func(ctx *v2.Context) {
				idx, file := ctx.Idx, ctx.File
				name := flatCallExpressionName(file, idx)
				if name != "read" {
					return
				}
				nodeText := file.FlatNodeText(idx)
				if !strings.Contains(nodeText, "FileInputStream") {
					return
				}
				if strings.Contains(nodeText, "buffered") {
					return
				}
				ctx.EmitAt(file.FlatRow(idx)+1, file.FlatCol(idx)+1, "FileInputStream.read() without BufferedInputStream; wrap in .buffered() for efficient reads.")
			},
		})
	}
	{
		r := &CursorLoopWithColumnIndexInLoopRule{BaseRule: BaseRule{RuleName: "CursorLoopWithColumnIndexInLoop", RuleSetName: "resource-cost", Sev: "warning", Desc: "Detects getColumnIndex() calls inside cursor while-loops that should be hoisted before the loop."}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"while_statement"}, Confidence: 0.75, OriginalV1: r,
			Check: func(ctx *v2.Context) {
				idx, file := ctx.Idx, ctx.File
				condText := ""
				for child := file.FlatFirstChild(idx); child != 0; child = file.FlatNextSib(child) {
					if file.FlatType(child) == "call_expression" || file.FlatType(child) == "navigation_expression" {
						condText = file.FlatNodeText(child)
						break
					}
				}
				if condText == "" {
					wholeText := file.FlatNodeText(idx)
					if !strings.Contains(wholeText, "moveToNext") {
						return
					}
				} else if !strings.Contains(condText, "moveToNext") {
					return
				}
				body, _ := file.FlatFindChild(idx, "statements")
				if body == 0 {
					for child := file.FlatFirstChild(idx); child != 0; child = file.FlatNextSib(child) {
						if file.FlatType(child) == "control_structure_body" {
							body = child
							break
						}
					}
				}
				if body == 0 {
					return
				}
				found := false
				file.FlatWalkNodes(body, "call_expression", func(callIdx uint32) {
					if found {
						return
					}
					if flatCallExpressionName(file, callIdx) == "getColumnIndex" || flatCallExpressionName(file, callIdx) == "getColumnIndexOrThrow" {
						found = true
					}
				})
				if !found {
					return
				}
				ctx.EmitAt(file.FlatRow(idx)+1, file.FlatCol(idx)+1, "cursor.getColumnIndex() inside while loop; hoist column index lookup before the loop.")
			},
		})
	}
	{
		r := &OkHttpClientCreatedPerCallRule{BaseRule: BaseRule{RuleName: "OkHttpClientCreatedPerCall", RuleSetName: "resource-cost", Sev: "warning", Desc: "Detects OkHttpClient construction in function bodies instead of reusing a singleton instance."}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"call_expression"}, Confidence: 0.75, OriginalV1: r,
			Check: func(ctx *v2.Context) {
				idx, file := ctx.Idx, ctx.File
				name := flatCallExpressionName(file, idx)
				nodeText := file.FlatNodeText(idx)
				isDirectConstruction := name == "OkHttpClient" && !strings.Contains(nodeText, "Builder")
				isBuilderBuild := name == "build" && strings.Contains(nodeText, "OkHttpClient")
				if !isDirectConstruction && !isBuilderBuild {
					return
				}
				if _, ok := flatEnclosingAncestor(file, idx, "object_declaration"); ok {
					return
				}
				if _, ok := flatEnclosingAncestor(file, idx, "companion_object"); ok {
					return
				}
				fn, ok := flatEnclosingFunction(file, idx)
				if !ok {
					return
				}
				if hasAnnotationFlat(file, fn, "Provides") || hasAnnotationFlat(file, fn, "Singleton") {
					return
				}
				prop, hasProp := flatEnclosingAncestor(file, idx, "property_declaration")
				if hasProp {
					propParent, ok := file.FlatParent(prop)
					if ok && file.FlatType(propParent) == "class_body" {
						return
					}
				}
				ctx.EmitAt(file.FlatRow(idx)+1, file.FlatCol(idx)+1, "OkHttpClient created in function body; reuse a singleton instance to share connection pools.")
			},
		})
	}
	{
		r := &OkHttpCallExecuteSyncRule{BaseRule: BaseRule{RuleName: "OkHttpCallExecuteSync", RuleSetName: "resource-cost", Sev: "warning", Desc: "Detects synchronous OkHttp Call.execute() inside suspend functions that block the coroutine thread."}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"call_expression"}, Confidence: 0.75, OriginalV1: r,
			Check: func(ctx *v2.Context) {
				idx, file := ctx.Idx, ctx.File
				name := flatCallExpressionName(file, idx)
				if name != "execute" {
					return
				}
				navExpr, _ := flatCallExpressionParts(file, idx)
				if navExpr == 0 {
					return
				}
				navText := file.FlatNodeText(navExpr)
				if !strings.Contains(navText, "newCall") && !strings.Contains(navText, "Call") {
					nodeText := file.FlatNodeText(idx)
					if !strings.Contains(nodeText, "execute()") {
						return
					}
					receiver := flatReceiverNameFromCall(file, idx)
					if receiver == "" {
						return
					}
					receiverLower := strings.ToLower(receiver)
					if !strings.Contains(receiverLower, "call") && !strings.Contains(receiverLower, "response") {
						return
					}
				}
				fn, ok := flatEnclosingFunction(file, idx)
				if !ok {
					return
				}
				if !file.FlatHasModifier(fn, "suspend") {
					return
				}
				ctx.EmitAt(file.FlatRow(idx)+1, file.FlatCol(idx)+1, "OkHttp Call.execute() in suspend function blocks the coroutine thread; use enqueue() or withContext(Dispatchers.IO).")
			},
		})
	}
	{
		r := &RetrofitCreateInHotPathRule{BaseRule: BaseRule{RuleName: "RetrofitCreateInHotPath", RuleSetName: "resource-cost", Sev: "warning", Desc: "Detects Retrofit.Builder().build().create() in function bodies instead of a singleton or @Provides."}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"call_expression"}, Confidence: 0.75, OriginalV1: r,
			Check: func(ctx *v2.Context) {
				idx, file := ctx.Idx, ctx.File
				name := flatCallExpressionName(file, idx)
				if name != "create" {
					return
				}
				nodeText := file.FlatNodeText(idx)
				if !strings.Contains(nodeText, "Retrofit") {
					return
				}
				if _, ok := flatEnclosingAncestor(file, idx, "object_declaration"); ok {
					return
				}
				if _, ok := flatEnclosingAncestor(file, idx, "companion_object"); ok {
					return
				}
				fn, ok := flatEnclosingFunction(file, idx)
				if !ok {
					return
				}
				if hasAnnotationFlat(file, fn, "Provides") || hasAnnotationFlat(file, fn, "Singleton") {
					return
				}
				ctx.EmitAt(file.FlatRow(idx)+1, file.FlatCol(idx)+1, "Retrofit.Builder().build().create() in function body; build and create the service once in a singleton or @Provides.")
			},
		})
	}
	{
		r := &HttpClientNotReusedRule{BaseRule: BaseRule{RuleName: "HttpClientNotReused", RuleSetName: "resource-cost", Sev: "warning", Desc: "Detects Java HttpClient.newHttpClient() in function bodies without singleton reuse."}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"call_expression"}, Confidence: 0.75, OriginalV1: r,
			Check: func(ctx *v2.Context) {
				idx, file := ctx.Idx, ctx.File
				name := flatCallExpressionName(file, idx)
				if name != "newHttpClient" && name != "newBuilder" {
					return
				}
				navExpr, _ := flatCallExpressionParts(file, idx)
				if navExpr == 0 {
					return
				}
				receiver := flatReceiverNameFromCall(file, idx)
				if receiver != "HttpClient" {
					return
				}
				if _, ok := flatEnclosingAncestor(file, idx, "object_declaration"); ok {
					return
				}
				if _, ok := flatEnclosingAncestor(file, idx, "companion_object"); ok {
					return
				}
				fn, ok := flatEnclosingFunction(file, idx)
				if !ok {
					return
				}
				if hasAnnotationFlat(file, fn, "Provides") || hasAnnotationFlat(file, fn, "Singleton") {
					return
				}
				ctx.EmitAt(file.FlatRow(idx)+1, file.FlatCol(idx)+1, "HttpClient.newHttpClient() in function body; reuse a singleton instance.")
			},
		})
	}
	{
		r := &DatabaseQueryOnMainThreadRule{BaseRule: BaseRule{RuleName: "DatabaseQueryOnMainThread", RuleSetName: "resource-cost", Sev: "warning", Desc: "Detects SQLiteDatabase query calls in non-suspend functions that may block the main thread."}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"call_expression"}, Confidence: 0.75, OriginalV1: r,
			Check: func(ctx *v2.Context) {
				idx, file := ctx.Idx, ctx.File
				name := flatCallExpressionName(file, idx)
				if !sqliteQueryMethods[name] {
					return
				}
				nodeText := file.FlatNodeText(idx)
				if !strings.Contains(nodeText, name+"(") {
					return
				}
				fn, ok := flatEnclosingFunction(file, idx)
				if !ok {
					return
				}
				if file.FlatHasModifier(fn, "suspend") {
					return
				}
				if _, ok := flatEnclosingAncestor(file, idx, "lambda_literal"); ok {
					fnBody := file.FlatNodeText(fn)
					if strings.Contains(fnBody, "withContext") || strings.Contains(fnBody, "Dispatchers.IO") {
						return
					}
				}
				ctx.EmitAt(file.FlatRow(idx)+1, file.FlatCol(idx)+1, fmt.Sprintf("SQLiteDatabase.%s() in non-suspend function may block the main thread; use withContext(Dispatchers.IO) or a suspend function.", name))
			},
		})
	}
	{
		r := &RoomLoadsAllWhereFirstUsedRule{BaseRule: BaseRule{RuleName: "RoomLoadsAllWhereFirstUsed", RuleSetName: "resource-cost", Sev: "warning", Desc: "Detects getAll().first() patterns that load an entire table for a single element instead of using LIMIT 1."}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"call_expression"}, Confidence: 0.75, OriginalV1: r,
			Check: func(ctx *v2.Context) {
				idx, file := ctx.Idx, ctx.File
				name := flatCallExpressionName(file, idx)
				if !loadAllTerminalMethods[name] {
					return
				}
				navExpr, _ := flatCallExpressionParts(file, idx)
				if navExpr == 0 {
					return
				}
				receiverText := ""
				for child := file.FlatFirstChild(navExpr); child != 0; child = file.FlatNextSib(child) {
					if file.FlatIsNamed(child) {
						receiverText = file.FlatNodeText(child)
						break
					}
				}
				if receiverText == "" {
					return
				}
				receiverCallName := ""
				for child := file.FlatFirstChild(navExpr); child != 0; child = file.FlatNextSib(child) {
					if file.FlatType(child) == "call_expression" {
						receiverCallName = flatCallExpressionName(file, child)
						break
					}
				}
				if !loadAllMethods[receiverCallName] {
					return
				}
				ctx.EmitAt(file.FlatRow(idx)+1, file.FlatCol(idx)+1, fmt.Sprintf("%s().%s() loads the entire table for a single element; add a LIMIT 1 query instead.", receiverCallName, name))
			},
		})
	}
	{
		r := &RecyclerAdapterWithoutDiffUtilRule{BaseRule: BaseRule{RuleName: "RecyclerAdapterWithoutDiffUtil", RuleSetName: "resource-cost", Sev: "warning", Desc: "Detects RecyclerView.Adapter subclasses using notifyDataSetChanged() without DiffUtil."}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"class_declaration"}, Confidence: 0.75, OriginalV1: r,
			Check: func(ctx *v2.Context) {
				idx, file := ctx.Idx, ctx.File
				nodeText := file.FlatNodeText(idx)
				if !strings.Contains(nodeText, "RecyclerView") || !strings.Contains(nodeText, "Adapter") {
					return
				}
				if strings.Contains(nodeText, "ListAdapter") {
					return
				}
				if !strings.Contains(nodeText, "notifyDataSetChanged") {
					return
				}
				if strings.Contains(nodeText, "DiffUtil") {
					return
				}
				name := extractIdentifierFlat(file, idx)
				if name == "" {
					name = "Adapter"
				}
				ctx.EmitAt(file.FlatRow(idx)+1, 1, fmt.Sprintf("'%s' uses notifyDataSetChanged() without DiffUtil; use ListAdapter or DiffUtil.calculateDiff() for efficient updates.", name))
			},
		})
	}
	{
		r := &RecyclerAdapterStableIdsDefaultRule{BaseRule: BaseRule{RuleName: "RecyclerAdapterStableIdsDefault", RuleSetName: "resource-cost", Sev: "info", Desc: "Detects RecyclerView.Adapter subclasses that do not enable stable IDs for better animation."}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"class_declaration"}, Confidence: 0.75, OriginalV1: r,
			Check: func(ctx *v2.Context) {
				idx, file := ctx.Idx, ctx.File
				nodeText := file.FlatNodeText(idx)
				if !strings.Contains(nodeText, "RecyclerView") || !strings.Contains(nodeText, "Adapter") {
					return
				}
				if strings.Contains(nodeText, "ListAdapter") {
					return
				}
				if strings.Contains(nodeText, "setHasStableIds") || strings.Contains(nodeText, "hasStableIds") {
					return
				}
				name := extractIdentifierFlat(file, idx)
				if name == "" {
					name = "Adapter"
				}
				ctx.EmitAt(file.FlatRow(idx)+1, 1, fmt.Sprintf("'%s' extends RecyclerView.Adapter without setHasStableIds(true); enable stable IDs for better animation and rebinding.", name))
			},
		})
	}
	{
		r := &LazyColumnInsideColumnRule{BaseRule: BaseRule{RuleName: "LazyColumnInsideColumn", RuleSetName: "resource-cost", Sev: "warning", Desc: "Detects LazyColumn or LazyRow nested inside a scrollable Column or Row causing measurement issues."}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"call_expression"}, Confidence: 0.75, OriginalV1: r,
			Check: func(ctx *v2.Context) {
				idx, file := ctx.Idx, ctx.File
				name := flatCallNameAny(file, idx)
				if name != "Column" && name != "Row" {
					return
				}
				nodeText := file.FlatNodeText(idx)
				isVertical := name == "Column"
				if isVertical {
					if !bytes.Contains([]byte(nodeText), []byte("verticalScroll")) {
						return
					}
					if !bytes.Contains([]byte(nodeText), lazyColumnToken) {
						return
					}
				} else {
					if !bytes.Contains([]byte(nodeText), []byte("horizontalScroll")) {
						return
					}
					if !bytes.Contains([]byte(nodeText), lazyRowToken) {
						return
					}
				}
				scrollDir := "verticalScroll"
				lazyChild := "LazyColumn"
				if !isVertical {
					scrollDir = "horizontalScroll"
					lazyChild = "LazyRow"
				}
				ctx.EmitAt(file.FlatRow(idx)+1, file.FlatCol(idx)+1, fmt.Sprintf("%s with %s contains %s; nested scroll containers cause measurement issues. Remove %s or replace %s with a regular list.", name, scrollDir, lazyChild, scrollDir, lazyChild))
			},
		})
	}
	{
		r := &RecyclerViewInLazyColumnRule{BaseRule: BaseRule{RuleName: "RecyclerViewInLazyColumn", RuleSetName: "resource-cost", Sev: "warning", Desc: "Detects AndroidView wrapping a RecyclerView inside a LazyColumn or LazyRow causing nested scrolling conflicts."}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"call_expression"}, Confidence: 0.75, OriginalV1: r,
			Check: func(ctx *v2.Context) {
				idx, file := ctx.Idx, ctx.File
				name := flatCallNameAny(file, idx)
				if name != "AndroidView" {
					return
				}
				nodeText := file.FlatNodeText(idx)
				if !strings.Contains(nodeText, "RecyclerView") {
					return
				}
				if !composeLambdaBelongsToCallFlat(file, idx, "items", "itemsIndexed", "item") {
					if _, ok := flatEnclosingAncestor(file, idx, "lambda_literal"); ok {
						parentText := ""
						if p, ok := flatEnclosingAncestor(file, idx, "call_expression"); ok {
							parentText = file.FlatNodeText(p)
						}
						if !strings.Contains(parentText, "LazyColumn") && !strings.Contains(parentText, "LazyRow") {
							return
						}
					} else {
						return
					}
				}
				ctx.EmitAt(file.FlatRow(idx)+1, file.FlatCol(idx)+1, "RecyclerView inside LazyColumn/LazyRow causes nested scrolling conflicts; use Compose lazy list items instead.")
			},
		})
	}
	{
		r := &ImageLoadedAtFullSizeInListRule{BaseRule: BaseRule{RuleName: "ImageLoadedAtFullSizeInList", RuleSetName: "resource-cost", Sev: "info", Desc: "Detects Glide or Coil image loading without size constraints in list item contexts."}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"call_expression"}, Confidence: 0.75, OriginalV1: r,
			Check: func(ctx *v2.Context) {
				idx, file := ctx.Idx, ctx.File
				name := flatCallExpressionName(file, idx)
				if name != "load" && name != "into" {
					return
				}
				nodeText := file.FlatNodeText(idx)
				isGlide := strings.Contains(nodeText, "Glide") || strings.Contains(nodeText, "RequestManager")
				isCoil := strings.Contains(nodeText, "ImageRequest") || strings.Contains(nodeText, "rememberAsyncImagePainter")
				if !isGlide && !isCoil {
					return
				}
				if strings.Contains(nodeText, "override(") || strings.Contains(nodeText, "size(") {
					return
				}
				inList := false
				if _, ok := flatEnclosingAncestor(file, idx, "class_declaration"); ok {
					classText := ""
					if cls, ok2 := flatEnclosingAncestor(file, idx, "class_declaration"); ok2 {
						classText = file.FlatNodeText(cls)
					}
					if strings.Contains(classText, "ViewHolder") || strings.Contains(classText, "RecyclerView") {
						inList = true
					}
				}
				if composeLambdaBelongsToCallFlat(file, idx, "items", "itemsIndexed", "item") {
					inList = true
				}
				if !inList {
					return
				}
				ctx.EmitAt(file.FlatRow(idx)+1, file.FlatCol(idx)+1, "Image loaded without size constraint in list context; use override() or size() to avoid decoding full-size bitmaps.")
			},
		})
	}
	{
		r := &ImageLoaderNoMemoryCacheRule{BaseRule: BaseRule{RuleName: "ImageLoaderNoMemoryCache", RuleSetName: "resource-cost", Sev: "info", Desc: "Detects image loaders configured to skip the memory cache, causing repeated decoding and GC pressure."}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"call_expression"}, Confidence: 0.75, OriginalV1: r,
			Check: func(ctx *v2.Context) {
				idx, file := ctx.Idx, ctx.File
				name := flatCallExpressionName(file, idx)
				nodeText := file.FlatNodeText(idx)
				if name == "skipMemoryCache" && strings.Contains(nodeText, "true") {
					ctx.EmitAt(file.FlatRow(idx)+1, file.FlatCol(idx)+1, "skipMemoryCache(true) disables the memory cache; this causes repeated decoding and GC pressure.")
					return
				}
				if name == "memoryCachePolicy" && strings.Contains(nodeText, "DISABLED") {
					ctx.EmitAt(file.FlatRow(idx)+1, file.FlatCol(idx)+1, "memoryCachePolicy(DISABLED) disables the memory cache; this causes repeated decoding and GC pressure.")
					return
				}
			},
		})
	}
	{
		r := &ComposePainterResourceInLoopRule{BaseRule: BaseRule{RuleName: "ComposePainterResourceInLoop", RuleSetName: "resource-cost", Sev: "warning", Desc: "Detects painterResource() calls inside list or loop lambdas that create a fresh painter per iteration."}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"call_expression"}, Confidence: 0.75, OriginalV1: r,
			Check: func(ctx *v2.Context) {
				idx, file := ctx.Idx, ctx.File
				name := flatCallExpressionName(file, idx)
				if name != "painterResource" {
					return
				}
				if resourceCostInsideLazyListLambda(file, idx) {
					ctx.EmitAt(file.FlatRow(idx)+1, file.FlatCol(idx)+1, "painterResource() inside list/loop lambda creates a fresh painter per iteration; hoist it outside the lambda.")
					return
				}
				if _, ok := flatEnclosingAncestor(file, idx, "for_statement"); ok {
					ctx.EmitAt(file.FlatRow(idx)+1, file.FlatCol(idx)+1, "painterResource() inside for loop creates a fresh painter per iteration; hoist it outside the loop.")
					return
				}
			},
		})
	}
	{
		r := &ComposeRememberInListRule{BaseRule: BaseRule{RuleName: "ComposeRememberInList", RuleSetName: "resource-cost", Sev: "warning", Desc: "Detects remember {} inside items {} without a key argument, causing recomputation on list reorder."}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"call_expression"}, Confidence: 0.75, OriginalV1: r,
			Check: func(ctx *v2.Context) {
				idx, file := ctx.Idx, ctx.File
				name := flatCallExpressionName(file, idx)
				if name != "remember" {
					return
				}
				if !resourceCostInsideLazyListLambda(file, idx) {
					return
				}
				args := flatCallKeyArguments(file, idx)
				if args != 0 {
					argCount := 0
					for child := file.FlatFirstChild(args); child != 0; child = file.FlatNextSib(child) {
						if file.FlatType(child) == "value_argument" {
							argCount++
						}
					}
					if argCount > 0 {
						return
					}
				}
				ctx.EmitAt(file.FlatRow(idx)+1, file.FlatCol(idx)+1, "remember {} inside items {} without a key causes recomputation on reorder; pass a key argument like remember(item) {}.")
			},
		})
	}
	{
		r := &PeriodicWorkRequestLessThan15MinRule{BaseRule: BaseRule{RuleName: "PeriodicWorkRequestLessThan15Min", RuleSetName: "resource-cost", Sev: "warning", Desc: "Detects PeriodicWorkRequest intervals below the 15-minute minimum enforced by WorkManager."}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"call_expression"}, Confidence: 0.75, OriginalV1: r,
			Check: func(ctx *v2.Context) {
				idx, file := ctx.Idx, ctx.File
				name := flatCallExpressionName(file, idx)
				if name != "PeriodicWorkRequestBuilder" && name != "PeriodicWorkRequest" {
					return
				}
				nodeText := file.FlatNodeText(idx)
				args := flatCallKeyArguments(file, idx)
				if args == 0 {
					return
				}
				intervalArg := flatPositionalValueArgument(file, args, 0)
				if intervalArg == 0 {
					return
				}
				argText := strings.TrimSpace(file.FlatNodeText(intervalArg))
				if strings.Contains(nodeText, "MINUTES") {
					if val, err := strconv.Atoi(argText); err == nil && val < 15 {
						ctx.EmitAt(file.FlatRow(idx)+1, file.FlatCol(idx)+1, fmt.Sprintf("PeriodicWorkRequest interval %d minutes is below the 15-minute minimum; WorkManager will coerce it to 15 minutes.", val))
						return
					}
				}
				if strings.Contains(nodeText, "SECONDS") {
					if val, err := strconv.Atoi(argText); err == nil && val < 900 {
						ctx.EmitAt(file.FlatRow(idx)+1, file.FlatCol(idx)+1, fmt.Sprintf("PeriodicWorkRequest interval %d seconds is below the 15-minute (900s) minimum; WorkManager will coerce it to 15 minutes.", val))
						return
					}
				}
			},
		})
	}
	{
		r := &WorkManagerNoBackoffRule{BaseRule: BaseRule{RuleName: "WorkManagerNoBackoff", RuleSetName: "resource-cost", Sev: "info", Desc: "Detects OneTimeWorkRequest chains without a setBackoffCriteria policy for retryable work."}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"call_expression"}, Confidence: 0.75, OriginalV1: r,
			Check: func(ctx *v2.Context) {
				idx, file := ctx.Idx, ctx.File
				name := flatCallExpressionName(file, idx)
				if name != "build" {
					return
				}
				nodeText := file.FlatNodeText(idx)
				if !strings.Contains(nodeText, "OneTimeWorkRequest") && !strings.Contains(nodeText, "OneTimeWorkRequestBuilder") {
					return
				}
				if strings.Contains(nodeText, "setBackoffCriteria") {
					return
				}
				ctx.EmitAt(file.FlatRow(idx)+1, file.FlatCol(idx)+1, "OneTimeWorkRequest without setBackoffCriteria; add a backoff policy for retry-able work.")
			},
		})
	}
	{
		r := &WorkManagerUniquePolicyKeepButReplaceIntendedRule{BaseRule: BaseRule{RuleName: "WorkManagerUniquePolicyKeepButReplaceIntended", RuleSetName: "resource-cost", Sev: "info", Desc: "Detects enqueueUniqueWork with KEEP policy followed by cancel logic where REPLACE may be intended."}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"call_expression"}, Confidence: 0.75, OriginalV1: r,
			Check: func(ctx *v2.Context) {
				idx, file := ctx.Idx, ctx.File
				name := flatCallExpressionName(file, idx)
				if name != "enqueueUniqueWork" && name != "enqueueUniquePeriodicWork" {
					return
				}
				nodeText := file.FlatNodeText(idx)
				if !strings.Contains(nodeText, "KEEP") {
					return
				}
				fnBody := ""
				if fn, ok := flatEnclosingFunction(file, idx); ok {
					fnBody = file.FlatNodeText(fn)
				}
				if fnBody == "" {
					return
				}
				if strings.Contains(fnBody, "cancelUniqueWork") || strings.Contains(fnBody, "cancelAllWork") {
					ctx.EmitAt(file.FlatRow(idx)+1, file.FlatCol(idx)+1, "enqueueUniqueWork with KEEP policy followed by cancel logic; REPLACE may be intended to restart the work.")
				}
			},
		})
	}
}
