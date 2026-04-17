package rules

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"

	"github.com/kaeawc/krit/internal/scanner"
)

var lazyListCallNames = map[string]bool{
	"items":          true,
	"itemsIndexed":   true,
	"item":           true,
	"forEach":        true,
	"forEachIndexed": true,
}

func resourceCostInsideLazyListLambda(file *scanner.File, idx uint32) bool {
	for cur, ok := file.FlatParent(idx); ok; cur, ok = file.FlatParent(cur) {
		if file.FlatType(cur) == "call_expression" {
			callName := flatCallNameAny(file, cur)
			if lazyListCallNames[callName] {
				return true
			}
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// Batch 1: In-Progress rules
// ---------------------------------------------------------------------------

// BufferedReadWithoutBufferRule detects FileInputStream.read(ByteArray(N))
// where N < 8192 without wrapping in BufferedInputStream.
type BufferedReadWithoutBufferRule struct {
	FlatDispatchBase
	BaseRule
}

func (r *BufferedReadWithoutBufferRule) Confidence() float64 { return 0.75 }
func (r *BufferedReadWithoutBufferRule) NodeTypes() []string { return []string{"call_expression"} }

func (r *BufferedReadWithoutBufferRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	name := flatCallExpressionName(file, idx)
	if name != "read" {
		return nil
	}

	nodeText := file.FlatNodeText(idx)
	if !strings.Contains(nodeText, "FileInputStream") {
		return nil
	}
	if strings.Contains(nodeText, "buffered") {
		return nil
	}

	return []scanner.Finding{r.Finding(
		file,
		file.FlatRow(idx)+1,
		file.FlatCol(idx)+1,
		"FileInputStream.read() without BufferedInputStream; wrap in .buffered() for efficient reads.",
	)}
}

// CursorLoopWithColumnIndexInLoopRule detects getColumnIndex() calls inside
// cursor.moveToNext() while loops.
type CursorLoopWithColumnIndexInLoopRule struct {
	FlatDispatchBase
	BaseRule
}

func (r *CursorLoopWithColumnIndexInLoopRule) Confidence() float64 { return 0.75 }
func (r *CursorLoopWithColumnIndexInLoopRule) NodeTypes() []string {
	return []string{"while_statement"}
}

func (r *CursorLoopWithColumnIndexInLoopRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
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
			return nil
		}
	} else if !strings.Contains(condText, "moveToNext") {
		return nil
	}

	body := file.FlatFindChild(idx, "statements")
	if body == 0 {
		for child := file.FlatFirstChild(idx); child != 0; child = file.FlatNextSib(child) {
			if file.FlatType(child) == "control_structure_body" {
				body = child
				break
			}
		}
	}
	if body == 0 {
		return nil
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
		return nil
	}

	return []scanner.Finding{r.Finding(
		file,
		file.FlatRow(idx)+1,
		file.FlatCol(idx)+1,
		"cursor.getColumnIndex() inside while loop; hoist column index lookup before the loop.",
	)}
}

// ---------------------------------------------------------------------------
// Batch 2: Network/IO rules
// ---------------------------------------------------------------------------

// OkHttpClientCreatedPerCallRule detects OkHttpClient() or
// OkHttpClient.Builder().build() in non-singleton function bodies.
type OkHttpClientCreatedPerCallRule struct {
	FlatDispatchBase
	BaseRule
}

func (r *OkHttpClientCreatedPerCallRule) Confidence() float64 { return 0.75 }
func (r *OkHttpClientCreatedPerCallRule) NodeTypes() []string { return []string{"call_expression"} }

func (r *OkHttpClientCreatedPerCallRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	name := flatCallExpressionName(file, idx)
	nodeText := file.FlatNodeText(idx)

	isDirectConstruction := name == "OkHttpClient" && !strings.Contains(nodeText, "Builder")
	isBuilderBuild := name == "build" && strings.Contains(nodeText, "OkHttpClient")

	if !isDirectConstruction && !isBuilderBuild {
		return nil
	}

	if _, ok := flatEnclosingAncestor(file, idx, "object_declaration"); ok {
		return nil
	}
	if _, ok := flatEnclosingAncestor(file, idx, "companion_object"); ok {
		return nil
	}

	fn, ok := flatEnclosingFunction(file, idx)
	if !ok {
		return nil
	}
	if hasAnnotationFlat(file, fn, "Provides") || hasAnnotationFlat(file, fn, "Singleton") {
		return nil
	}

	prop, hasProp := flatEnclosingAncestor(file, idx, "property_declaration")
	if hasProp {
		propParent, ok := file.FlatParent(prop)
		if ok && file.FlatType(propParent) == "class_body" {
			return nil
		}
	}

	return []scanner.Finding{r.Finding(
		file,
		file.FlatRow(idx)+1,
		file.FlatCol(idx)+1,
		"OkHttpClient created in function body; reuse a singleton instance to share connection pools.",
	)}
}

// OkHttpCallExecuteSyncRule detects Call.execute() inside suspend functions
// where enqueue() with a callback should be used instead.
type OkHttpCallExecuteSyncRule struct {
	FlatDispatchBase
	BaseRule
}

func (r *OkHttpCallExecuteSyncRule) Confidence() float64 { return 0.75 }
func (r *OkHttpCallExecuteSyncRule) NodeTypes() []string { return []string{"call_expression"} }

func (r *OkHttpCallExecuteSyncRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	name := flatCallExpressionName(file, idx)
	if name != "execute" {
		return nil
	}

	navExpr, _ := flatCallExpressionParts(file, idx)
	if navExpr == 0 {
		return nil
	}

	navText := file.FlatNodeText(navExpr)
	if !strings.Contains(navText, "newCall") && !strings.Contains(navText, "Call") {
		nodeText := file.FlatNodeText(idx)
		if !strings.Contains(nodeText, "execute()") {
			return nil
		}
		receiver := flatReceiverNameFromCall(file, idx)
		if receiver == "" {
			return nil
		}
		receiverLower := strings.ToLower(receiver)
		if !strings.Contains(receiverLower, "call") && !strings.Contains(receiverLower, "response") {
			return nil
		}
	}

	fn, ok := flatEnclosingFunction(file, idx)
	if !ok {
		return nil
	}
	if !file.FlatHasModifier(fn, "suspend") {
		return nil
	}

	return []scanner.Finding{r.Finding(
		file,
		file.FlatRow(idx)+1,
		file.FlatCol(idx)+1,
		"OkHttp Call.execute() in suspend function blocks the coroutine thread; use enqueue() or withContext(Dispatchers.IO).",
	)}
}

// RetrofitCreateInHotPathRule detects Retrofit.Builder()...build().create()
// in non-init, non-object function bodies.
type RetrofitCreateInHotPathRule struct {
	FlatDispatchBase
	BaseRule
}

func (r *RetrofitCreateInHotPathRule) Confidence() float64 { return 0.75 }
func (r *RetrofitCreateInHotPathRule) NodeTypes() []string { return []string{"call_expression"} }

func (r *RetrofitCreateInHotPathRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	name := flatCallExpressionName(file, idx)
	if name != "create" {
		return nil
	}

	nodeText := file.FlatNodeText(idx)
	if !strings.Contains(nodeText, "Retrofit") {
		return nil
	}

	if _, ok := flatEnclosingAncestor(file, idx, "object_declaration"); ok {
		return nil
	}
	if _, ok := flatEnclosingAncestor(file, idx, "companion_object"); ok {
		return nil
	}
	fn, ok := flatEnclosingFunction(file, idx)
	if !ok {
		return nil
	}
	if hasAnnotationFlat(file, fn, "Provides") || hasAnnotationFlat(file, fn, "Singleton") {
		return nil
	}

	return []scanner.Finding{r.Finding(
		file,
		file.FlatRow(idx)+1,
		file.FlatCol(idx)+1,
		"Retrofit.Builder().build().create() in function body; build and create the service once in a singleton or @Provides.",
	)}
}

// HttpClientNotReusedRule detects Java HttpClient.newHttpClient() in function
// bodies without caching.
type HttpClientNotReusedRule struct {
	FlatDispatchBase
	BaseRule
}

func (r *HttpClientNotReusedRule) Confidence() float64 { return 0.75 }
func (r *HttpClientNotReusedRule) NodeTypes() []string { return []string{"call_expression"} }

func (r *HttpClientNotReusedRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	name := flatCallExpressionName(file, idx)
	if name != "newHttpClient" && name != "newBuilder" {
		return nil
	}

	navExpr, _ := flatCallExpressionParts(file, idx)
	if navExpr == 0 {
		return nil
	}
	receiver := flatReceiverNameFromCall(file, idx)
	if receiver != "HttpClient" {
		return nil
	}

	if _, ok := flatEnclosingAncestor(file, idx, "object_declaration"); ok {
		return nil
	}
	if _, ok := flatEnclosingAncestor(file, idx, "companion_object"); ok {
		return nil
	}
	fn, ok := flatEnclosingFunction(file, idx)
	if !ok {
		return nil
	}
	if hasAnnotationFlat(file, fn, "Provides") || hasAnnotationFlat(file, fn, "Singleton") {
		return nil
	}

	return []scanner.Finding{r.Finding(
		file,
		file.FlatRow(idx)+1,
		file.FlatCol(idx)+1,
		"HttpClient.newHttpClient() in function body; reuse a singleton instance.",
	)}
}

// ---------------------------------------------------------------------------
// Batch 3: Database rules
// ---------------------------------------------------------------------------

// DatabaseQueryOnMainThreadRule detects SQLiteDatabase.rawQuery()/query()
// calls in non-suspend functions without withContext.
type DatabaseQueryOnMainThreadRule struct {
	FlatDispatchBase
	BaseRule
}

func (r *DatabaseQueryOnMainThreadRule) Confidence() float64 { return 0.75 }
func (r *DatabaseQueryOnMainThreadRule) NodeTypes() []string { return []string{"call_expression"} }

var sqliteQueryMethods = map[string]bool{
	"rawQuery": true,
	"query":    true,
	"execSQL":  true,
}

func (r *DatabaseQueryOnMainThreadRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	name := flatCallExpressionName(file, idx)
	if !sqliteQueryMethods[name] {
		return nil
	}

	nodeText := file.FlatNodeText(idx)
	if !strings.Contains(nodeText, name+"(") {
		return nil
	}

	fn, ok := flatEnclosingFunction(file, idx)
	if !ok {
		return nil
	}

	if file.FlatHasModifier(fn, "suspend") {
		return nil
	}

	if _, ok := flatEnclosingAncestor(file, idx, "lambda_literal"); ok {
		fnBody := file.FlatNodeText(fn)
		if strings.Contains(fnBody, "withContext") || strings.Contains(fnBody, "Dispatchers.IO") {
			return nil
		}
	}

	return []scanner.Finding{r.Finding(
		file,
		file.FlatRow(idx)+1,
		file.FlatCol(idx)+1,
		fmt.Sprintf("SQLiteDatabase.%s() in non-suspend function may block the main thread; use withContext(Dispatchers.IO) or a suspend function.", name),
	)}
}

// RoomLoadsAllWhereFirstUsedRule detects dao.getAll().first() or similar
// patterns that load an entire table for a single element.
type RoomLoadsAllWhereFirstUsedRule struct {
	FlatDispatchBase
	BaseRule
}

func (r *RoomLoadsAllWhereFirstUsedRule) Confidence() float64 { return 0.75 }
func (r *RoomLoadsAllWhereFirstUsedRule) NodeTypes() []string { return []string{"call_expression"} }

var loadAllTerminalMethods = map[string]bool{
	"first":        true,
	"firstOrNull":  true,
	"single":       true,
	"singleOrNull": true,
	"last":         true,
	"lastOrNull":   true,
}

var loadAllMethods = map[string]bool{
	"getAll":    true,
	"findAll":   true,
	"loadAll":   true,
	"fetchAll":  true,
	"queryAll":  true,
	"selectAll": true,
}

func (r *RoomLoadsAllWhereFirstUsedRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	name := flatCallExpressionName(file, idx)
	if !loadAllTerminalMethods[name] {
		return nil
	}

	navExpr, _ := flatCallExpressionParts(file, idx)
	if navExpr == 0 {
		return nil
	}

	receiverText := ""
	for child := file.FlatFirstChild(navExpr); child != 0; child = file.FlatNextSib(child) {
		if file.FlatIsNamed(child) {
			receiverText = file.FlatNodeText(child)
			break
		}
	}

	if receiverText == "" {
		return nil
	}

	receiverCallName := ""
	for child := file.FlatFirstChild(navExpr); child != 0; child = file.FlatNextSib(child) {
		if file.FlatType(child) == "call_expression" {
			receiverCallName = flatCallExpressionName(file, child)
			break
		}
	}

	if !loadAllMethods[receiverCallName] {
		return nil
	}

	return []scanner.Finding{r.Finding(
		file,
		file.FlatRow(idx)+1,
		file.FlatCol(idx)+1,
		fmt.Sprintf("%s().%s() loads the entire table for a single element; add a LIMIT 1 query instead.", receiverCallName, name),
	)}
}

// ---------------------------------------------------------------------------
// Batch 4: RecyclerView/List rules
// ---------------------------------------------------------------------------

// RecyclerAdapterWithoutDiffUtilRule detects RecyclerView.Adapter subclasses
// using notifyDataSetChanged() without DiffUtil or ListAdapter.
type RecyclerAdapterWithoutDiffUtilRule struct {
	FlatDispatchBase
	BaseRule
}

func (r *RecyclerAdapterWithoutDiffUtilRule) Confidence() float64 { return 0.75 }
func (r *RecyclerAdapterWithoutDiffUtilRule) NodeTypes() []string {
	return []string{"class_declaration"}
}

func (r *RecyclerAdapterWithoutDiffUtilRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	nodeText := file.FlatNodeText(idx)
	if !strings.Contains(nodeText, "RecyclerView") || !strings.Contains(nodeText, "Adapter") {
		return nil
	}
	if strings.Contains(nodeText, "ListAdapter") {
		return nil
	}
	if !strings.Contains(nodeText, "notifyDataSetChanged") {
		return nil
	}
	if strings.Contains(nodeText, "DiffUtil") {
		return nil
	}

	name := extractIdentifierFlat(file, idx)
	if name == "" {
		name = "Adapter"
	}

	return []scanner.Finding{r.Finding(
		file,
		file.FlatRow(idx)+1,
		1,
		fmt.Sprintf("'%s' uses notifyDataSetChanged() without DiffUtil; use ListAdapter or DiffUtil.calculateDiff() for efficient updates.", name),
	)}
}

// RecyclerAdapterStableIdsDefaultRule detects RecyclerView.Adapter subclasses
// that don't call setHasStableIds(true) and don't extend ListAdapter.
type RecyclerAdapterStableIdsDefaultRule struct {
	FlatDispatchBase
	BaseRule
}

func (r *RecyclerAdapterStableIdsDefaultRule) Confidence() float64 { return 0.75 }
func (r *RecyclerAdapterStableIdsDefaultRule) NodeTypes() []string {
	return []string{"class_declaration"}
}

func (r *RecyclerAdapterStableIdsDefaultRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	nodeText := file.FlatNodeText(idx)
	if !strings.Contains(nodeText, "RecyclerView") || !strings.Contains(nodeText, "Adapter") {
		return nil
	}
	if strings.Contains(nodeText, "ListAdapter") {
		return nil
	}
	if strings.Contains(nodeText, "setHasStableIds") || strings.Contains(nodeText, "hasStableIds") {
		return nil
	}

	name := extractIdentifierFlat(file, idx)
	if name == "" {
		name = "Adapter"
	}

	return []scanner.Finding{r.Finding(
		file,
		file.FlatRow(idx)+1,
		1,
		fmt.Sprintf("'%s' extends RecyclerView.Adapter without setHasStableIds(true); enable stable IDs for better animation and rebinding.", name),
	)}
}

var lazyColumnToken = []byte("LazyColumn")
var lazyRowToken = []byte("LazyRow")

// LazyColumnInsideColumnRule detects LazyColumn nested inside a Column with
// verticalScroll modifier.
type LazyColumnInsideColumnRule struct {
	FlatDispatchBase
	BaseRule
}

func (r *LazyColumnInsideColumnRule) Confidence() float64 { return 0.75 }
func (r *LazyColumnInsideColumnRule) NodeTypes() []string { return []string{"call_expression"} }

func (r *LazyColumnInsideColumnRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	name := flatCallNameAny(file, idx)
	if name != "Column" && name != "Row" {
		return nil
	}

	nodeText := file.FlatNodeText(idx)
	isVertical := name == "Column"

	if isVertical {
		if !bytes.Contains([]byte(nodeText), []byte("verticalScroll")) {
			return nil
		}
		if !bytes.Contains([]byte(nodeText), lazyColumnToken) {
			return nil
		}
	} else {
		if !bytes.Contains([]byte(nodeText), []byte("horizontalScroll")) {
			return nil
		}
		if !bytes.Contains([]byte(nodeText), lazyRowToken) {
			return nil
		}
	}

	scrollDir := "verticalScroll"
	lazyChild := "LazyColumn"
	if !isVertical {
		scrollDir = "horizontalScroll"
		lazyChild = "LazyRow"
	}

	return []scanner.Finding{r.Finding(
		file,
		file.FlatRow(idx)+1,
		file.FlatCol(idx)+1,
		fmt.Sprintf("%s with %s contains %s; nested scroll containers cause measurement issues. Remove %s or replace %s with a regular list.", name, scrollDir, lazyChild, scrollDir, lazyChild),
	)}
}

// RecyclerViewInLazyColumnRule detects AndroidView wrapping a RecyclerView
// inside a LazyColumn/LazyRow.
type RecyclerViewInLazyColumnRule struct {
	FlatDispatchBase
	BaseRule
}

func (r *RecyclerViewInLazyColumnRule) Confidence() float64 { return 0.75 }
func (r *RecyclerViewInLazyColumnRule) NodeTypes() []string { return []string{"call_expression"} }

func (r *RecyclerViewInLazyColumnRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	name := flatCallNameAny(file, idx)
	if name != "AndroidView" {
		return nil
	}

	nodeText := file.FlatNodeText(idx)
	if !strings.Contains(nodeText, "RecyclerView") {
		return nil
	}

	if !composeLambdaBelongsToCallFlat(file, idx, "items", "itemsIndexed", "item") {
		if _, ok := flatEnclosingAncestor(file, idx, "lambda_literal"); ok {
			parentText := ""
			if p, ok := flatEnclosingAncestor(file, idx, "call_expression"); ok {
				parentText = file.FlatNodeText(p)
			}
			if !strings.Contains(parentText, "LazyColumn") && !strings.Contains(parentText, "LazyRow") {
				return nil
			}
		} else {
			return nil
		}
	}

	return []scanner.Finding{r.Finding(
		file,
		file.FlatRow(idx)+1,
		file.FlatCol(idx)+1,
		"RecyclerView inside LazyColumn/LazyRow causes nested scrolling conflicts; use Compose lazy list items instead.",
	)}
}

// ---------------------------------------------------------------------------
// Batch 5: Image Loading rules
// ---------------------------------------------------------------------------

// ImageLoadedAtFullSizeInListRule detects Glide/Coil image loading without
// size constraints in list item contexts.
type ImageLoadedAtFullSizeInListRule struct {
	FlatDispatchBase
	BaseRule
}

func (r *ImageLoadedAtFullSizeInListRule) Confidence() float64 { return 0.75 }
func (r *ImageLoadedAtFullSizeInListRule) NodeTypes() []string { return []string{"call_expression"} }

func (r *ImageLoadedAtFullSizeInListRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	name := flatCallExpressionName(file, idx)
	if name != "load" && name != "into" {
		return nil
	}

	nodeText := file.FlatNodeText(idx)
	isGlide := strings.Contains(nodeText, "Glide") || strings.Contains(nodeText, "RequestManager")
	isCoil := strings.Contains(nodeText, "ImageRequest") || strings.Contains(nodeText, "rememberAsyncImagePainter")

	if !isGlide && !isCoil {
		return nil
	}

	if strings.Contains(nodeText, "override(") || strings.Contains(nodeText, "size(") {
		return nil
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
		return nil
	}

	return []scanner.Finding{r.Finding(
		file,
		file.FlatRow(idx)+1,
		file.FlatCol(idx)+1,
		"Image loaded without size constraint in list context; use override() or size() to avoid decoding full-size bitmaps.",
	)}
}

// ImageLoaderNoMemoryCacheRule detects image loaders configured to skip
// the memory cache.
type ImageLoaderNoMemoryCacheRule struct {
	FlatDispatchBase
	BaseRule
}

func (r *ImageLoaderNoMemoryCacheRule) Confidence() float64 { return 0.75 }
func (r *ImageLoaderNoMemoryCacheRule) NodeTypes() []string { return []string{"call_expression"} }

func (r *ImageLoaderNoMemoryCacheRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	name := flatCallExpressionName(file, idx)
	nodeText := file.FlatNodeText(idx)

	if name == "skipMemoryCache" && strings.Contains(nodeText, "true") {
		return []scanner.Finding{r.Finding(
			file,
			file.FlatRow(idx)+1,
			file.FlatCol(idx)+1,
			"skipMemoryCache(true) disables the memory cache; this causes repeated decoding and GC pressure.",
		)}
	}

	if name == "memoryCachePolicy" && strings.Contains(nodeText, "DISABLED") {
		return []scanner.Finding{r.Finding(
			file,
			file.FlatRow(idx)+1,
			file.FlatCol(idx)+1,
			"memoryCachePolicy(DISABLED) disables the memory cache; this causes repeated decoding and GC pressure.",
		)}
	}

	return nil
}

// ---------------------------------------------------------------------------
// Batch 6: Compose rules
// ---------------------------------------------------------------------------

// ComposePainterResourceInLoopRule detects painterResource() inside
// forEach/items lambda bodies.
type ComposePainterResourceInLoopRule struct {
	FlatDispatchBase
	BaseRule
}

func (r *ComposePainterResourceInLoopRule) Confidence() float64 { return 0.75 }
func (r *ComposePainterResourceInLoopRule) NodeTypes() []string { return []string{"call_expression"} }

func (r *ComposePainterResourceInLoopRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	name := flatCallExpressionName(file, idx)
	if name != "painterResource" {
		return nil
	}

	if resourceCostInsideLazyListLambda(file, idx) {
		return []scanner.Finding{r.Finding(
			file,
			file.FlatRow(idx)+1,
			file.FlatCol(idx)+1,
			"painterResource() inside list/loop lambda creates a fresh painter per iteration; hoist it outside the lambda.",
		)}
	}

	if _, ok := flatEnclosingAncestor(file, idx, "for_statement"); ok {
		return []scanner.Finding{r.Finding(
			file,
			file.FlatRow(idx)+1,
			file.FlatCol(idx)+1,
			"painterResource() inside for loop creates a fresh painter per iteration; hoist it outside the loop.",
		)}
	}

	return nil
}

// ComposeRememberInListRule detects remember{} inside items{} lambda
// without a key argument — causes recomputation on list reordering.
type ComposeRememberInListRule struct {
	FlatDispatchBase
	BaseRule
}

func (r *ComposeRememberInListRule) Confidence() float64 { return 0.75 }
func (r *ComposeRememberInListRule) NodeTypes() []string { return []string{"call_expression"} }

func (r *ComposeRememberInListRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	name := flatCallExpressionName(file, idx)
	if name != "remember" {
		return nil
	}

	if !resourceCostInsideLazyListLambda(file, idx) {
		return nil
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
			return nil
		}
	}

	return []scanner.Finding{r.Finding(
		file,
		file.FlatRow(idx)+1,
		file.FlatCol(idx)+1,
		"remember {} inside items {} without a key causes recomputation on reorder; pass a key argument like remember(item) {}.",
	)}
}

// ---------------------------------------------------------------------------
// Batch 7: WorkManager rules
// ---------------------------------------------------------------------------

// PeriodicWorkRequestLessThan15MinRule detects PeriodicWorkRequestBuilder
// with an interval less than 15 minutes.
type PeriodicWorkRequestLessThan15MinRule struct {
	FlatDispatchBase
	BaseRule
}

func (r *PeriodicWorkRequestLessThan15MinRule) Confidence() float64 { return 0.75 }
func (r *PeriodicWorkRequestLessThan15MinRule) NodeTypes() []string {
	return []string{"call_expression"}
}

func (r *PeriodicWorkRequestLessThan15MinRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	name := flatCallExpressionName(file, idx)
	if name != "PeriodicWorkRequestBuilder" && name != "PeriodicWorkRequest" {
		return nil
	}

	nodeText := file.FlatNodeText(idx)

	args := flatCallKeyArguments(file, idx)
	if args == 0 {
		return nil
	}

	intervalArg := flatPositionalValueArgument(file, args, 0)
	if intervalArg == 0 {
		return nil
	}
	argText := strings.TrimSpace(file.FlatNodeText(intervalArg))

	if strings.Contains(nodeText, "MINUTES") {
		if val, err := strconv.Atoi(argText); err == nil && val < 15 {
			return []scanner.Finding{r.Finding(
				file,
				file.FlatRow(idx)+1,
				file.FlatCol(idx)+1,
				fmt.Sprintf("PeriodicWorkRequest interval %d minutes is below the 15-minute minimum; WorkManager will coerce it to 15 minutes.", val),
			)}
		}
	}

	if strings.Contains(nodeText, "SECONDS") {
		if val, err := strconv.Atoi(argText); err == nil && val < 900 {
			return []scanner.Finding{r.Finding(
				file,
				file.FlatRow(idx)+1,
				file.FlatCol(idx)+1,
				fmt.Sprintf("PeriodicWorkRequest interval %d seconds is below the 15-minute (900s) minimum; WorkManager will coerce it to 15 minutes.", val),
			)}
		}
	}

	return nil
}

// WorkManagerNoBackoffRule detects OneTimeWorkRequestBuilder chains without
// setBackoffCriteria.
type WorkManagerNoBackoffRule struct {
	FlatDispatchBase
	BaseRule
}

func (r *WorkManagerNoBackoffRule) Confidence() float64 { return 0.75 }
func (r *WorkManagerNoBackoffRule) NodeTypes() []string { return []string{"call_expression"} }

func (r *WorkManagerNoBackoffRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	name := flatCallExpressionName(file, idx)
	if name != "build" {
		return nil
	}

	nodeText := file.FlatNodeText(idx)
	if !strings.Contains(nodeText, "OneTimeWorkRequest") && !strings.Contains(nodeText, "OneTimeWorkRequestBuilder") {
		return nil
	}
	if strings.Contains(nodeText, "setBackoffCriteria") {
		return nil
	}

	return []scanner.Finding{r.Finding(
		file,
		file.FlatRow(idx)+1,
		file.FlatCol(idx)+1,
		"OneTimeWorkRequest without setBackoffCriteria; add a backoff policy for retry-able work.",
	)}
}

// WorkManagerUniquePolicyKeepButReplaceIntendedRule detects enqueueUniqueWork
// with ExistingWorkPolicy.KEEP where REPLACE may be intended.
type WorkManagerUniquePolicyKeepButReplaceIntendedRule struct {
	FlatDispatchBase
	BaseRule
}

func (r *WorkManagerUniquePolicyKeepButReplaceIntendedRule) Confidence() float64 { return 0.75 }
func (r *WorkManagerUniquePolicyKeepButReplaceIntendedRule) NodeTypes() []string {
	return []string{"call_expression"}
}

func (r *WorkManagerUniquePolicyKeepButReplaceIntendedRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	name := flatCallExpressionName(file, idx)
	if name != "enqueueUniqueWork" && name != "enqueueUniquePeriodicWork" {
		return nil
	}

	nodeText := file.FlatNodeText(idx)
	if !strings.Contains(nodeText, "KEEP") {
		return nil
	}

	fnBody := ""
	if fn, ok := flatEnclosingFunction(file, idx); ok {
		fnBody = file.FlatNodeText(fn)
	}
	if fnBody == "" {
		return nil
	}
	if strings.Contains(fnBody, "cancelUniqueWork") || strings.Contains(fnBody, "cancelAllWork") {
		return []scanner.Finding{r.Finding(
			file,
			file.FlatRow(idx)+1,
			file.FlatCol(idx)+1,
			"enqueueUniqueWork with KEEP policy followed by cancel logic; REPLACE may be intended to restart the work.",
		)}
	}

	return nil
}
