package rules

import (
	"fmt"
	"strings"

	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

func registerResourceCostRules() {

	// --- from resource_cost.go ---
	{
		r := &BufferedReadWithoutBufferRule{BaseRule: BaseRule{RuleName: "BufferedReadWithoutBuffer", RuleSetName: "resource-cost", Sev: "info", Desc: "Detects FileInputStream.read() without BufferedInputStream wrapping for efficient reads."}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			NodeTypes: []string{"call_expression", "method_invocation"}, Languages: []scanner.Language{scanner.LangKotlin, scanner.LangJava}, Confidence: api.ConfidenceMedium, Implementation: r,
			Check: func(ctx *api.Context) {
				idx, file := ctx.Idx, ctx.File
				name := databaseCallName(file, idx)
				if name != "read" {
					return
				}
				if file.FlatType(idx) == "method_invocation" {
					if !bufferedReadJavaFileInputStreamEvidence(file, idx) {
						return
					}
				} else if !callReceiverConstructedOrTyped(ctx, idx, "FileInputStream", "java.io.FileInputStream", "FileInputStream") {
					return
				}
				if receiverContainsCallName(file, idx, "buffered", "BufferedInputStream") {
					return
				}
				// Reads into a buffer of 8 KiB or more are already efficient.
				if bufferedReadFillsLargeBuffer(file, idx) {
					return
				}
				ctx.EmitAt(file.FlatRow(idx)+1, file.FlatCol(idx)+1, "FileInputStream.read() without BufferedInputStream; wrap in .buffered() for efficient reads.")
			},
		})
	}
	{
		r := &CursorLoopWithColumnIndexInLoopRule{BaseRule: BaseRule{RuleName: "CursorLoopWithColumnIndexInLoop", RuleSetName: "resource-cost", Sev: "warning", Desc: "Detects getColumnIndex() calls inside cursor while-loops that should be hoisted before the loop."}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			NodeTypes: []string{"while_statement"}, Languages: []scanner.Language{scanner.LangKotlin, scanner.LangJava}, Confidence: api.ConfidenceMedium, Implementation: r,
			Check: func(ctx *api.Context) {
				idx, file := ctx.Idx, ctx.File
				if !whileConditionHasCallName(file, idx, "moveToNext") {
					return
				}
				body, _ := file.FlatFindChild(idx, "statements")
				if body == 0 {
					for child := file.FlatFirstChild(idx); child != 0; child = file.FlatNextSib(child) {
						if file.FlatType(child) == "control_structure_body" || file.FlatType(child) == "block" {
							body = child
							break
						}
					}
				}
				if body == 0 {
					return
				}
				var foundNode uint32
				foundName := ""
				loopLocals := cursorLoopLocalNames(file, body)
				file.FlatWalkNodes(body, "call_expression", func(callIdx uint32) {
					if foundNode != 0 {
						return
					}
					if name := flatCallExpressionName(file, callIdx); (name == "getColumnIndex" || name == "getColumnIndexOrThrow") && cursorColumnArgIsLoopInvariant(file, callIdx, loopLocals) {
						foundNode, foundName = callIdx, name
					}
				})
				file.FlatWalkNodes(body, "method_invocation", func(callIdx uint32) {
					if foundNode != 0 {
						return
					}
					if name := databaseCallName(file, callIdx); (name == "getColumnIndex" || name == "getColumnIndexOrThrow") && cursorColumnArgIsLoopInvariant(file, callIdx, loopLocals) {
						foundNode, foundName = callIdx, name
					}
				})
				if foundNode == 0 {
					return
				}
				receiver := "cursor"
				if node := cursorCallReceiverNode(file, foundNode); node != 0 {
					receiver = strings.TrimSpace(file.FlatNodeText(node))
				}
				ctx.EmitAt(callLine(file, foundNode), callCol(file, foundNode), receiver+"."+foundName+"() inside while loop; hoist column index lookup before the loop.")
			},
		})
	}
	{
		r := &OkHTTPClientCreatedPerCallRule{BaseRule: BaseRule{RuleName: "OkHttpClientCreatedPerCall", RuleSetName: "resource-cost", Sev: "warning", Desc: "Detects OkHttpClient construction in function bodies instead of reusing a singleton instance."}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			NodeTypes: []string{"call_expression", "method_invocation", "object_creation_expression"}, Languages: []scanner.Language{scanner.LangKotlin, scanner.LangJava}, Confidence: api.ConfidenceMedium, Implementation: r,
			Check: func(ctx *api.Context) {
				idx, file := ctx.Idx, ctx.File
				if scanner.IsTestFile(file.Path) || okHTTPClientToolingPath(file.Path) {
					return
				}
				name := databaseCallName(file, idx)
				nodeText := file.FlatNodeText(idx)
				receiver := semanticsReceiverNode(ctx, idx)
				if receiver == 0 {
					receiver = astReceiverNodeFromCall(file, idx)
				}
				isDirectConstruction := name == "OkHttpClient"
				isBuilderBuild := name == "build" && receiver != 0 && receiverChainHasQualifiedRoot(file, receiver, "OkHttpClient")
				if !okHTTPClientConstructionLooksReal(file, idx, name, nodeText) &&
					(!semanticCallTargetOrReceiverType(ctx, idx,
						[]string{"okhttp3.OkHttpClient", "okhttp3.OkHttpClient.Builder"},
						[]string{"okhttp3.OkHttpClient", "okhttp3.OkHttpClient.Builder", "OkHttpClient", "Builder"},
					) || (!isDirectConstruction && !isBuilderBuild)) {
					return
				}
				if _, ok := flatEnclosingAncestor(file, idx, "object_declaration"); ok {
					return
				}
				if _, ok := flatEnclosingAncestor(file, idx, "companion_object"); ok {
					return
				}
				fn, ok := flatEnclosingCallable(file, idx)
				if !ok {
					return
				}
				if hasExactAnnotationFlat(file, fn, "Provides") || hasExactAnnotationFlat(file, fn, "Singleton") {
					return
				}
				if okHTTPClientCreationAssignedToStaticField(file, idx) {
					return
				}
				if okHTTPClientFeedsOneShotInitializer(file, idx, fn) {
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
		r := &OkHTTPCallExecuteSyncRule{BaseRule: BaseRule{RuleName: "OkHttpCallExecuteSync", RuleSetName: "resource-cost", Sev: "warning", Desc: "Detects synchronous OkHttp Call.execute() inside suspend functions that block the coroutine thread."}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			NodeTypes: []string{"call_expression"}, Languages: []scanner.Language{scanner.LangKotlin, scanner.LangJava}, Confidence: api.ConfidenceMedium, Implementation: r,
			Check: func(ctx *api.Context) {
				idx, file := ctx.Idx, ctx.File
				name := flatCallExpressionName(file, idx)
				if name != "execute" {
					return
				}
				semanticOkHTTPCall := semanticCallTargetOrReceiverType(ctx, idx,
					[]string{"okhttp3.Call"},
					[]string{"okhttp3.Call", "Call"},
				) || receiverContainsCallName(file, idx, "newCall") ||
					callReceiverParameterHasType(ctx, idx, "okhttp3.Call", "Call")
				fn, ok := flatEnclosingFunction(file, idx)
				if !ok {
					return
				}
				if !file.FlatHasModifier(fn, "suspend") {
					return
				}
				if !semanticOkHTTPCall && !okHTTPExecuteCallLooksBlocking(file, idx, fn) {
					return
				}
				if databaseQueryInsideBackgroundBoundary(file, idx) {
					return
				}
				if okHTTPExecuteIsCancellableBridge(file, idx) || functionKdocRequiresIODispatcher(file, fn) {
					return
				}
				ctx.EmitAtNode(calleeNameAnchor(file, idx), "Synchronous OkHttp Call.execute() in a suspend function has no IO dispatcher evidence; wrap the call in withContext(Dispatchers.IO) or use an async OkHttp/Retrofit adapter.")
			},
		})
	}
	{
		r := &RetrofitCreateInHotPathRule{BaseRule: BaseRule{RuleName: "RetrofitCreateInHotPath", RuleSetName: "resource-cost", Sev: "warning", Desc: "Detects Retrofit.Builder().build().create() in function bodies instead of a singleton or @Provides."}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			NodeTypes: []string{"call_expression", "method_invocation"}, Languages: []scanner.Language{scanner.LangKotlin, scanner.LangJava}, Confidence: api.ConfidenceMedium, Implementation: r,
			Check: func(ctx *api.Context) {
				idx, file := ctx.Idx, ctx.File
				name := databaseCallName(file, idx)
				if name != "create" {
					return
				}
				nodeText := file.FlatNodeText(idx)
				if !retrofitCreateLooksReal(file, idx, nodeText) && !semanticCallTargetOrReceiverType(ctx, idx,
					[]string{"retrofit2.Retrofit"},
					[]string{"retrofit2.Retrofit", "Retrofit"},
				) {
					return
				}
				if _, ok := flatEnclosingAncestor(file, idx, "object_declaration"); ok {
					return
				}
				if _, ok := flatEnclosingAncestor(file, idx, "companion_object"); ok {
					return
				}
				fn, ok := flatEnclosingCallable(file, idx)
				if !ok {
					return
				}
				if hasExactAnnotationFlat(file, fn, "Provides") || hasExactAnnotationFlat(file, fn, "Singleton") {
					return
				}
				ctx.EmitAt(file.FlatRow(idx)+1, file.FlatCol(idx)+1, "Retrofit.Builder().build().create() in function body; build and create the service once in a singleton or @Provides.")
			},
		})
	}
	{
		r := &HTTPClientNotReusedRule{BaseRule: BaseRule{RuleName: "HttpClientNotReused", RuleSetName: "resource-cost", Sev: "warning", Desc: "Detects Java HttpClient.newHttpClient() in function bodies without singleton reuse."}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			NodeTypes: []string{"call_expression", "method_invocation"}, Languages: []scanner.Language{scanner.LangKotlin, scanner.LangJava}, Confidence: api.ConfidenceMedium, Implementation: r,
			Check: func(ctx *api.Context) {
				idx, file := ctx.Idx, ctx.File
				if scanner.IsTestFile(file.Path) || okHTTPClientToolingPath(file.Path) {
					return
				}
				name := databaseCallName(file, idx)
				if name != "newHttpClient" && name != "newBuilder" {
					return
				}
				if !javaHTTPClientCallLooksReal(file, idx) {
					return
				}
				if _, ok := flatEnclosingAncestor(file, idx, "object_declaration"); ok {
					return
				}
				if _, ok := flatEnclosingAncestor(file, idx, "companion_object"); ok {
					return
				}
				fn, ok := flatEnclosingCallable(file, idx)
				if !ok {
					return
				}
				if hasExactAnnotationFlat(file, fn, "Provides") || hasExactAnnotationFlat(file, fn, "Singleton") {
					return
				}
				ctx.EmitAt(calleeLine(file, idx), calleeCol(file, idx), "HttpClient.newHttpClient() in function body; reuse a singleton instance.")
			},
		})
	}
	{
		r := &DatabaseQueryOnMainThreadRule{BaseRule: BaseRule{RuleName: "DatabaseQueryOnMainThread", RuleSetName: "resource-cost", Sev: "warning", Desc: "Detects SQLiteDatabase query calls in code with positive main-thread evidence."}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			Needs: api.NeedsParsedFiles, Confidence: api.ConfidenceMedium, Implementation: r,
			NeedsLibraryFacts: true,
			Check:             r.checkParsedFiles,
		})
	}
	{
		r := &RoomLoadsAllWhereFirstUsedRule{BaseRule: BaseRule{RuleName: "RoomLoadsAllWhereFirstUsed", RuleSetName: "resource-cost", Sev: "warning", Desc: "Detects getAll().first() patterns that load an entire table for a single element instead of using LIMIT 1."}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			Needs: api.NeedsParsedFiles, Confidence: api.ConfidenceMedium, Implementation: r,
			NeedsLibraryFacts: true,
			Check:             r.checkParsedFiles,
		})
	}
	{
		r := &RecyclerAdapterWithoutDiffUtilRule{BaseRule: BaseRule{RuleName: "RecyclerAdapterWithoutDiffUtil", RuleSetName: "resource-cost", Sev: "warning", Desc: "Detects RecyclerView.Adapter subclasses using notifyDataSetChanged() without DiffUtil."}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			NodeTypes:  []string{"class_declaration"},
			Needs:      api.NeedsTypeInfo | api.NeedsOracleSupertypes,
			Languages:  []scanner.Language{scanner.LangKotlin, scanner.LangJava},
			Confidence: 0.75, Implementation: r,
			Check: func(ctx *api.Context) {
				idx, file := ctx.Idx, ctx.File
				if scanner.IsTestFile(file.Path) || !isRecyclerAdapterClassFlat(ctx, idx) {
					return
				}
				if classExtendsAnyFlat(file, idx, "androidx.recyclerview.widget.ListAdapter", "ListAdapter") {
					return
				}
				// android.widget.BaseAdapter feeds ListView, which DiffUtil does not support.
				if !classIsRecyclerViewAdapterFlat(file, idx) {
					return
				}
				if !subtreeHasCallName(file, idx, "notifyDataSetChanged") {
					return
				}
				call := firstUnsuppressedCall(file, idx, "notifyDataSetChanged", "NotifyDataSetChanged")
				if call == 0 {
					return
				}
				if subtreeHasReferenceName(file, idx, "DiffUtil", "ListAdapter") {
					return
				}
				name := extractIdentifierFlat(file, idx)
				if name == "" {
					name = "Adapter"
				}
				ctx.EmitAtNode(calleeIdentifierNode(file, call), fmt.Sprintf("'%s' uses notifyDataSetChanged() without DiffUtil; use ListAdapter or DiffUtil.calculateDiff() for efficient updates.", name))
			},
		})
	}
	{
		r := &RecyclerAdapterStableIDsDefaultRule{BaseRule: BaseRule{RuleName: "RecyclerAdapterStableIdsDefault", RuleSetName: "resource-cost", Sev: "info", Desc: "Detects RecyclerView.Adapter subclasses that do not enable stable IDs for better animation."}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			NodeTypes:  []string{"class_declaration"},
			Needs:      api.NeedsTypeInfo | api.NeedsOracleSupertypes,
			Languages:  []scanner.Language{scanner.LangKotlin, scanner.LangJava},
			Confidence: 0.75, Implementation: r,
			Check: func(ctx *api.Context) {
				idx, file := ctx.Idx, ctx.File
				if !isRecyclerAdapterClassFlat(ctx, idx) {
					return
				}
				if classExtendsAnyFlat(file, idx, "androidx.recyclerview.widget.ListAdapter", "ListAdapter") {
					return
				}
				if subtreeHasCallName(file, idx, "setHasStableIds", "hasStableIds") {
					return
				}
				name := extractIdentifierFlat(file, idx)
				if name == "" {
					name = "Adapter"
				}
				ctx.EmitAt(file.FlatRow(idx)+1, file.FlatCol(idx)+1, fmt.Sprintf("'%s' extends RecyclerView.Adapter without setHasStableIds(true); enable stable IDs for better animation and rebinding.", name))
			},
		})
	}
	{
		r := &LazyColumnInsideColumnRule{BaseRule: BaseRule{RuleName: "LazyColumnInsideColumn", RuleSetName: "resource-cost", Sev: "warning", Desc: "Detects LazyColumn or LazyRow nested inside a scrollable Column or Row causing measurement issues."}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			NodeTypes: []string{"call_expression"}, Confidence: api.ConfidenceMedium, Implementation: r,
			Check: func(ctx *api.Context) {
				idx, file := ctx.Idx, ctx.File
				name := flatCallNameAny(file, idx)
				if name != "Column" && name != "Row" {
					return
				}
				isVertical := name == "Column"
				scrollCall, lazyCall := "horizontalScroll", "LazyRow"
				boundNames := []string{"width", "size", "widthIn", "requiredWidth", "requiredSize"}
				if isVertical {
					scrollCall, lazyCall = "verticalScroll", "LazyColumn"
					boundNames = []string{"height", "size", "heightIn", "requiredHeight", "requiredSize", "fillParentMaxHeight", "aspectRatio"}
				}
				// The scroll modifier must be applied to this container, not to a nested one.
				ownArgs := flatCallKeyArguments(file, idx)
				if ownArgs == 0 || !subtreeHasCallName(file, ownArgs, scrollCall) {
					return
				}
				unbounded := false
				file.FlatWalkNodes(idx, "call_expression", func(lazy uint32) {
					if unbounded || flatCallNameAny(file, lazy) != lazyCall {
						return
					}
					// A fixed-size lazy list gets finite constraints and is allowed inside a scrolling parent.
					if lazyArgs := flatCallKeyArguments(file, lazy); lazyArgs != 0 && subtreeHasCallName(file, lazyArgs, boundNames...) {
						return
					}
					unbounded = true
				})
				if !unbounded {
					return
				}
				scrollDir := "verticalScroll"
				lazyChild := "LazyColumn"
				if !isVertical {
					scrollDir = "horizontalScroll"
					lazyChild = "LazyRow"
				}
				ctx.EmitAt(callLine(file, idx), callCol(file, idx), fmt.Sprintf("%s with %s contains %s; nested scroll containers cause measurement issues. Remove %s or replace %s with a regular list.", name, scrollDir, lazyChild, scrollDir, lazyChild))
			},
		})
	}
	{
		r := &RecyclerViewInLazyColumnRule{BaseRule: BaseRule{RuleName: "RecyclerViewInLazyColumn", RuleSetName: "resource-cost", Sev: "warning", Desc: "Detects AndroidView wrapping a RecyclerView inside a LazyColumn or LazyRow causing nested scrolling conflicts."}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			NodeTypes: []string{"call_expression"}, Confidence: api.ConfidenceMedium, Implementation: r,
			Check: func(ctx *api.Context) {
				idx, file := ctx.Idx, ctx.File
				name := flatCallNameAny(file, idx)
				if name != "AndroidView" {
					return
				}
				if !subtreeHasReferenceName(file, idx, "RecyclerView") {
					return
				}
				if !composeLambdaBelongsToCallFlat(file, idx, "items", "itemsIndexed", "item") {
					if _, ok := flatEnclosingAncestor(file, idx, "lambda_literal"); ok {
						if p, ok := flatEnclosingAncestor(file, idx, "call_expression"); ok {
							if !subtreeHasCallName(file, p, "LazyColumn", "LazyRow") {
								return
							}
						} else {
							return
						}
					} else {
						return
					}
				}
				ctx.EmitAt(callLine(file, idx), callCol(file, idx), "RecyclerView inside LazyColumn/LazyRow causes nested scrolling conflicts; use Compose lazy list items instead.")
			},
		})
	}
	{
		r := &ImageLoadedAtFullSizeInListRule{BaseRule: BaseRule{RuleName: "ImageLoadedAtFullSizeInList", RuleSetName: "resource-cost", Sev: "info", Desc: "Detects Glide or Coil image loading without size constraints in list item contexts."}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			NodeTypes:  []string{"call_expression"},
			Needs:      api.NeedsTypeInfo | api.NeedsOracleSupertypes,
			Confidence: 0.75, Implementation: r,
			Check: func(ctx *api.Context) {
				idx, file := ctx.Idx, ctx.File
				name := flatCallExpressionName(file, idx)
				if name != "load" && name != "into" {
					return
				}
				receiver := semanticsReceiverNode(ctx, idx)
				if receiver == 0 {
					receiver = astReceiverNodeFromCall(file, idx)
				}
				isGlide := (receiver != 0 && receiverChainHasQualifiedRoot(file, receiver, "Glide")) ||
					callReceiverConstructedOrTyped(ctx, idx, "RequestManager", "com.bumptech.glide.RequestManager", "RequestManager")
				isCoil := receiverContainsCallName(file, idx, "ImageRequest", "rememberAsyncImagePainter")
				if !isGlide && !isCoil {
					return
				}
				if receiverContainsCallName(file, idx, "override", "size") || subtreeHasCallName(file, idx, "override", "size") {
					return
				}
				inList := false
				if _, ok := flatEnclosingAncestor(file, idx, "class_declaration"); ok {
					if cls, ok2 := flatEnclosingAncestor(file, idx, "class_declaration"); ok2 {
						if classHasNestedViewHolderFlat(file, cls) ||
							classExtendsAnyFlat(file, cls, "RecyclerView.ViewHolder", "ViewHolder") ||
							isRecyclerAdapterClassFlat(ctx, cls) {
							inList = true
						}
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
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			NodeTypes: []string{"call_expression"}, Confidence: api.ConfidenceMedium, Implementation: r,
			Check: func(ctx *api.Context) {
				idx, file := ctx.Idx, ctx.File
				name := flatCallExpressionName(file, idx)
				if name == "skipMemoryCache" && callHasBooleanArgument(file, idx, true) {
					ctx.EmitAt(callLine(file, idx), callCol(file, idx), "skipMemoryCache(true) disables the memory cache; this causes repeated decoding and GC pressure.")
					return
				}
				if name == "memoryCachePolicy" && callHasReferenceArgument(file, idx, "DISABLED") {
					ctx.EmitAt(callLine(file, idx), callCol(file, idx), "memoryCachePolicy(DISABLED) disables the memory cache; this causes repeated decoding and GC pressure.")
					return
				}
			},
		})
	}
	{
		r := &ComposePainterResourceInLoopRule{BaseRule: BaseRule{RuleName: "ComposePainterResourceInLoop", RuleSetName: "resource-cost", Sev: "warning", Desc: "Detects painterResource() calls inside list or loop lambdas that create a fresh painter per iteration."}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			NodeTypes: []string{"call_expression"}, Confidence: api.ConfidenceMedium, Implementation: r,
			Check: func(ctx *api.Context) {
				idx, file := ctx.Idx, ctx.File
				name := flatCallExpressionName(file, idx)
				if name != "painterResource" {
					return
				}
				// A resource that depends on the iteration variable cannot be hoisted out of the loop.
				bound, enclosed := resourceCostLoopBoundNames(file, idx, resourceCostLoopCallNames, true)
				if !enclosed {
					return
				}
				args := flatCallKeyArguments(file, idx)
				if args == 0 || resourceCostSubtreeUsesName(file, args, bound) {
					return
				}
				// A branch inside the loop paints only some items, so hoisting is not free.
				if resourceCostGuardedInsideLoop(file, idx) {
					return
				}
				// A slot lambda (an icon or content parameter) is composed on its own schedule, not once per iteration.
				kind := resourceCostDirectLoopKind(file, idx)
				if kind == "" {
					return
				}
				// A plain inline loop over a few children is not worth hoisting out of.
				if resourceCostInlineLoopCallNames[kind] && !resourceCostInsideLazyItemsLambda(file, idx) {
					return
				}
				if resourceCostInsideLoopLambda(file, idx) {
					ctx.EmitAt(callLine(file, idx), callCol(file, idx), "painterResource() inside list/loop lambda creates a fresh painter per iteration; hoist it outside the lambda.")
					return
				}
				if _, ok := flatEnclosingAncestor(file, idx, "for_statement"); ok {
					ctx.EmitAt(callLine(file, idx), callCol(file, idx), "painterResource() inside for loop creates a fresh painter per iteration; hoist it outside the loop.")
					return
				}
			},
		})
	}
	{
		r := &ComposeRememberInListRule{BaseRule: BaseRule{RuleName: "ComposeRememberInList", RuleSetName: "resource-cost", Sev: "warning", Desc: "Detects remember {} inside items {} without a key argument, causing recomputation on list reorder."}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			NodeTypes: []string{"call_expression"}, Confidence: api.ConfidenceMedium, Implementation: r,
			Check: func(ctx *api.Context) {
				idx, file := ctx.Idx, ctx.File
				name := flatCallExpressionName(file, idx)
				if name != "remember" {
					return
				}
				// Only the content lambda of items/itemsIndexed re-runs per item; item { } runs once.
				bound, enclosed := resourceCostLoopBoundNames(file, idx, rememberInListCallNames, false)
				if !enclosed {
					return
				}
				if body := rememberInListBody(file, idx); body == 0 || !resourceCostSubtreeUsesName(file, body, bound) {
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
				ctx.EmitAt(callLine(file, idx), callCol(file, idx), "remember {} inside items {} without a key causes recomputation on reorder; pass a key argument like remember(item) {}.")
			},
		})
	}
	{
		r := &PeriodicWorkRequestLessThan15MinRule{BaseRule: BaseRule{RuleName: "PeriodicWorkRequestLessThan15Min", RuleSetName: "resource-cost", Sev: "warning", Desc: "Detects PeriodicWorkRequest intervals below the 15-minute minimum enforced by WorkManager."}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			NodeTypes: []string{"call_expression"}, Confidence: api.ConfidenceMedium, Implementation: r,
			Check: func(ctx *api.Context) {
				idx, file := ctx.Idx, ctx.File
				name := flatCallExpressionName(file, idx)
				if name != "PeriodicWorkRequestBuilder" && name != "PeriodicWorkRequest" {
					return
				}
				args := flatCallKeyArguments(file, idx)
				if args == 0 {
					return
				}
				millis, ok := workManagerIntervalMillis(file, args)
				if !ok || millis >= workManagerMinPeriodicMillis {
					return
				}
				ctx.EmitAt(callLine(file, idx), callCol(file, idx), fmt.Sprintf("PeriodicWorkRequest interval %s is below the 15-minute minimum; WorkManager will coerce it to 15 minutes.", workManagerIntervalLabel(millis)))
			},
		})
	}
	{
		r := &WorkManagerNoBackoffRule{BaseRule: BaseRule{RuleName: "WorkManagerNoBackoff", RuleSetName: "resource-cost", Sev: "info", Desc: "Detects OneTimeWorkRequest chains without a setBackoffCriteria policy for retryable work."}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			NodeTypes: []string{"call_expression"}, Confidence: api.ConfidenceMedium, Implementation: r,
			Check: func(ctx *api.Context) {
				idx, file := ctx.Idx, ctx.File
				name := flatCallExpressionName(file, idx)
				if name != "build" {
					return
				}
				if !receiverContainsCallName(file, idx, "OneTimeWorkRequest", "OneTimeWorkRequestBuilder") &&
					!semanticCallTargetOrReceiverType(ctx, idx,
						[]string{"androidx.work.OneTimeWorkRequest.Builder"},
						[]string{"androidx.work.OneTimeWorkRequest.Builder", "OneTimeWorkRequest.Builder"},
					) {
					return
				}
				if receiverContainsCallName(file, idx, "setBackoffCriteria") {
					return
				}
				ctx.EmitAt(callLine(file, idx), callCol(file, idx), "OneTimeWorkRequest without setBackoffCriteria; add a backoff policy for retry-able work.")
			},
		})
	}
	{
		r := &WorkManagerUniquePolicyKeepButReplaceIntendedRule{BaseRule: BaseRule{RuleName: "WorkManagerUniquePolicyKeepButReplaceIntended", RuleSetName: "resource-cost", Sev: "info", Desc: "Detects enqueueUniqueWork with KEEP policy followed by cancel logic where REPLACE may be intended."}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			NodeTypes: []string{"call_expression"}, Confidence: api.ConfidenceMedium, Implementation: r,
			Check: func(ctx *api.Context) {
				idx, file := ctx.Idx, ctx.File
				name := flatCallExpressionName(file, idx)
				if name != "enqueueUniqueWork" && name != "enqueueUniquePeriodicWork" {
					return
				}
				if !callHasReferenceArgument(file, idx, "KEEP") {
					return
				}
				if fn, ok := flatEnclosingFunction(file, idx); ok {
					if subtreeHasCallName(file, fn, "cancelUniqueWork", "cancelAllWork") {
						ctx.EmitAt(callLine(file, idx), callCol(file, idx), "enqueueUniqueWork with KEEP policy followed by cancel logic; REPLACE may be intended to restart the work.")
					}
				}
			},
		})
	}
}
