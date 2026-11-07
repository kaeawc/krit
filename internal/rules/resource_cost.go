package rules

import (
	"path/filepath"
	"strconv"
	"strings"

	"github.com/kaeawc/krit/internal/javafacts"
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// resourceCostLoopCallNames is the set of lambda-receiving calls whose body
// runs once per element. The singular LazyListScope.item { } is deliberately
// excluded: it renders exactly once, so building a painter inside it is not a
// per-iteration cost. The plural items/itemsIndexed, the for-each family, and
// the common iteration combinators are all genuine loops.
var resourceCostLoopCallNames = map[string]bool{
	"items":          true,
	"itemsIndexed":   true,
	"forEach":        true,
	"forEachIndexed": true,
	"map":            true,
	"mapIndexed":     true,
	"repeat":         true,
}

// resourceCostInsideLoopLambda reports whether idx sits inside a lambda body
// that is executed per iteration. Unlike resourceCostInsideLazyListLambda it
// does not treat the singular LazyListScope.item { } as a loop.
func resourceCostInsideLoopLambda(file *scanner.File, idx uint32) bool {
	for cur, ok := file.FlatParent(idx); ok; cur, ok = file.FlatParent(cur) {
		if file.FlatType(cur) == "call_expression" {
			callName := flatCallNameAny(file, cur)
			if resourceCostLoopCallNames[callName] {
				return true
			}
		}
	}
	return false
}

// resourceCostInlineLoopCallNames are inline iteration helpers; outside a lazy list they run a fixed, small number of times.
var resourceCostInlineLoopCallNames = map[string]bool{"forEach": true, "forEachIndexed": true, "map": true, "mapIndexed": true, "repeat": true}

// resourceCostDirectLoopKind returns the loop whose body directly holds idx: the name of the iteration call
// whose lambda is the nearest enclosing lambda, or "for" for a for loop. It returns "" when a slot or other
// lambda sits between idx and the loop.
func resourceCostDirectLoopKind(file *scanner.File, idx uint32) string {
	for cur, ok := file.FlatParent(idx); ok; cur, ok = file.FlatParent(cur) {
		switch file.FlatType(cur) {
		case "for_statement":
			return "for"
		case "lambda_literal":
			for name := range resourceCostLoopCallNames {
				if composeLambdaBelongsToCallFlat(file, cur, name) {
					return name
				}
			}
			return ""
		}
	}
	return ""
}

// resourceCostInsideLazyItemsLambda reports whether idx sits in an items/itemsIndexed/item lambda.
func resourceCostInsideLazyItemsLambda(file *scanner.File, idx uint32) bool {
	for cur, ok := file.FlatParent(idx); ok; cur, ok = file.FlatParent(cur) {
		if file.FlatType(cur) == "call_expression" {
			switch flatCallNameAny(file, cur) {
			case "items", "itemsIndexed", "item":
				return true
			}
		}
	}
	return false
}

// resourceCostGuardedInsideLoop reports a call under an if or when branch that sits inside the loop,
// where it runs for some iterations only.
func resourceCostGuardedInsideLoop(file *scanner.File, idx uint32) bool {
	guarded := false
	for cur, ok := file.FlatParent(idx); ok; cur, ok = file.FlatParent(cur) {
		switch file.FlatType(cur) {
		case "if_expression", "when_entry", "when_expression":
			guarded = true
		case "for_statement":
			return guarded
		case "call_expression":
			if resourceCostLoopCallNames[flatCallNameAny(file, cur)] {
				return guarded
			}
		}
	}
	return false
}

// resourceCostLoopBoundNames returns the names bound per iteration by the lambdas of calls named
// callNames that enclose idx: explicit parameters (implicit `it` when none) plus locals declared
// before idx in that lambda. With includeForLoops it adds `for` loop variables. The bool reports
// whether any such lambda or loop encloses idx.
func resourceCostLoopBoundNames(file *scanner.File, idx uint32, callNames map[string]bool, includeForLoops bool) (map[string]bool, bool) {
	names := map[string]bool{}
	enclosed := false
	var calls []string
	for name := range callNames {
		calls = append(calls, name)
	}
	for cur, ok := file.FlatParent(idx); ok; cur, ok = file.FlatParent(cur) {
		switch file.FlatType(cur) {
		case "lambda_literal":
			if !composeLambdaBelongsToCallFlat(file, cur, calls...) {
				continue
			}
			enclosed = true
			params := composeLambdaParameterNamesFlat(file, cur)
			if len(params) == 0 {
				names["it"] = true
			}
			for _, p := range params {
				names[p] = true
			}
			file.FlatWalkNodes(cur, "property_declaration", func(prop uint32) {
				if file.FlatStartByte(prop) < file.FlatStartByte(idx) {
					if n := composePropertyNameFlat(file, prop); n != "" {
						names[n] = true
					}
				}
			})
		case "for_statement":
			if !includeForLoops {
				continue
			}
			enclosed = true
			if decl, ok := file.FlatFindChild(cur, "variable_declaration"); ok {
				if ident, ok := file.FlatFindChild(decl, "simple_identifier"); ok {
					names[file.FlatNodeText(ident)] = true
				}
			}
		}
	}
	return names, enclosed
}

// resourceCostSubtreeUsesName reports whether any identifier under root is in names.
func resourceCostSubtreeUsesName(file *scanner.File, root uint32, names map[string]bool) bool {
	found := false
	file.FlatWalkNodes(root, "simple_identifier", func(ident uint32) {
		if !found && names[file.FlatNodeText(ident)] {
			found = true
		}
	})
	return found
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

func (r *BufferedReadWithoutBufferRule) Confidence() float64 { return api.ConfidenceMedium }

// CursorLoopWithColumnIndexInLoopRule detects getColumnIndex() calls inside
// cursor.moveToNext() while loops.
type CursorLoopWithColumnIndexInLoopRule struct {
	FlatDispatchBase
	BaseRule
}

func (r *CursorLoopWithColumnIndexInLoopRule) Confidence() float64 { return api.ConfidenceMedium }

// ---------------------------------------------------------------------------
// Batch 2: Network/IO rules
// ---------------------------------------------------------------------------

// OkHTTPClientCreatedPerCallRule detects OkHttpClient() or
// OkHttpClient.Builder().build() in non-singleton function bodies.
type OkHTTPClientCreatedPerCallRule struct {
	FlatDispatchBase
	BaseRule
}

func (r *OkHTTPClientCreatedPerCallRule) Confidence() float64 { return api.ConfidenceMedium }

// OkHTTPCallExecuteSyncRule detects Call.execute() inside suspend functions
// where enqueue() with a callback should be used instead.
type OkHTTPCallExecuteSyncRule struct {
	FlatDispatchBase
	BaseRule
}

func (r *OkHTTPCallExecuteSyncRule) Confidence() float64 { return api.ConfidenceMedium }

func okHTTPExecuteCallLooksBlocking(file *scanner.File, idx, fn uint32) bool {
	receiver := databaseCallReceiverName(file, idx)
	if receiver == "" {
		return false
	}
	receiverLower := strings.ToLower(receiver)
	if receiverLower == "call" || strings.HasSuffix(receiverLower, "call") || strings.Contains(receiverLower, "okhttp") {
		return true
	}
	return okHTTPReceiverDeclaredAsCall(file, fn, receiver)
}

func flatEnclosingCallable(file *scanner.File, idx uint32) (uint32, bool) {
	return flatEnclosingAncestor(file, idx, "function_declaration", "method_declaration")
}

func sourceImportsOrMentions(file *scanner.File, fqn string) bool {
	if file == nil {
		return false
	}
	if file.Language == scanner.LangJava {
		// javafacts maintains its own per-file source-fact cache, so the
		// per-(file, fqn) memo we used to keep here was redundant.
		return javafacts.SourceFactsForFile(file).ImportsOrMentions(fqn)
	}
	if fileFactsCache().Imports(file).HasFQN(fqn) {
		return true
	}
	// Fallback for non-import mentions (string literals, doc references,
	// etc.). Cheap on byte slices; not worth a per-pair memo now that
	// the import-set is shared.
	return fileFactsCache().Mentions(file, fqn)
}

func okHTTPClientConstructionLooksReal(file *scanner.File, idx uint32, name, nodeText string) bool {
	if !sourceImportsOrMentions(file, "okhttp3.OkHttpClient") {
		return false
	}
	if name == "" {
		name = databaseCallName(file, idx)
	}
	compact := strings.Join(strings.Fields(nodeText), "")
	switch file.FlatType(idx) {
	case "object_creation_expression":
		return strings.Contains(compact, "newOkHttpClient(") ||
			strings.Contains(compact, "newokhttp3.OkHttpClient(")
	case "method_invocation":
		return name == "build" && strings.Contains(compact, "OkHttpClient.Builder")
	}
	isDirectConstruction := name == "OkHttpClient" && !strings.Contains(nodeText, "Builder")
	isBuilderBuild := name == "build" && strings.Contains(nodeText, "OkHttpClient")
	return isDirectConstruction || isBuilderBuild
}

func bufferedReadJavaFileInputStreamEvidence(file *scanner.File, idx uint32) bool {
	if !sourceImportsOrMentions(file, "java.io.FileInputStream") {
		return false
	}
	receiverText := javaMethodReceiverText(file, idx)
	if strings.Contains(receiverText, "BufferedInputStream") {
		return false
	}
	if strings.Contains(receiverText, "FileInputStream") {
		return true
	}
	receiver := databaseCallReceiverName(file, idx)
	if receiver == "" {
		return false
	}
	fn, ok := flatEnclosingCallable(file, idx)
	if !ok {
		return false
	}
	found := false
	file.FlatWalkNodes(fn, "local_variable_declaration", func(decl uint32) {
		if found || file.FlatStartByte(decl) >= file.FlatStartByte(idx) {
			return
		}
		text := file.FlatNodeText(decl)
		if !strings.Contains(text, receiver) {
			return
		}
		if strings.Contains(text, "BufferedInputStream") {
			return
		}
		if strings.Contains(text, "FileInputStream") {
			found = true
		}
	})
	return found
}

func okHTTPClientCreationAssignedToStaticField(file *scanner.File, idx uint32) bool {
	for current, ok := file.FlatParent(idx); ok; current, ok = file.FlatParent(current) {
		switch file.FlatType(current) {
		case "assignment_expression":
			text := file.FlatNodeText(current)
			eq := strings.Index(text, "=")
			if eq < 0 {
				return false
			}
			fieldName := strings.TrimSpace(text[:eq])
			if dot := strings.LastIndex(fieldName, "."); dot >= 0 {
				fieldName = strings.TrimSpace(fieldName[dot+1:])
			}
			if fieldName == "" || strings.ContainsAny(fieldName, " \t\n()") {
				return false
			}
			classIdx, ok := flatEnclosingAncestor(file, current, "class_declaration", "object_declaration")
			if !ok {
				return false
			}
			classText := file.FlatNodeText(classIdx)
			return strings.Contains(classText, "static") && strings.Contains(classText, fieldName+";")
		case "function_declaration", "method_declaration", "source_file":
			return false
		}
	}
	return false
}

func retrofitCreateLooksReal(file *scanner.File, _ uint32, nodeText string) bool {
	if !sourceImportsOrMentions(file, "retrofit2.Retrofit") {
		return false
	}
	return strings.Contains(nodeText, "Retrofit") || strings.Contains(nodeText, "retrofit2.Retrofit")
}

func javaHTTPClientCallLooksReal(file *scanner.File, idx uint32) bool {
	if !sourceImportsOrMentions(file, "java.net.http.HttpClient") {
		return false
	}
	receiver := databaseCallReceiverName(file, idx)
	return receiver == "HttpClient" || strings.HasSuffix(receiver, ".HttpClient")
}

func okHTTPReceiverDeclaredAsCall(file *scanner.File, fn uint32, receiver string) bool {
	if receiver == "" {
		return false
	}
	header := file.FlatNodeText(fn)
	if bodyIdx := strings.Index(header, "{"); bodyIdx >= 0 {
		header = header[:bodyIdx]
	}
	header = strings.Join(strings.Fields(header), "")
	return strings.Contains(header, receiver+":okhttp3.Call") ||
		strings.Contains(header, receiver+":Call") ||
		strings.Contains(header, receiver+":OkHttpCall")
}

// RetrofitCreateInHotPathRule detects Retrofit.Builder()...build().create()
// in non-init, non-object function bodies.
type RetrofitCreateInHotPathRule struct {
	FlatDispatchBase
	BaseRule
}

func (r *RetrofitCreateInHotPathRule) Confidence() float64 { return api.ConfidenceMedium }

// HTTPClientNotReusedRule detects Java HttpClient.newHttpClient() in function
// bodies without caching.
type HTTPClientNotReusedRule struct {
	FlatDispatchBase
	BaseRule
}

func (r *HTTPClientNotReusedRule) Confidence() float64 { return api.ConfidenceMedium }

func databaseCallName(file *scanner.File, call uint32) string {
	if file == nil || call == 0 {
		return ""
	}
	switch file.FlatType(call) {
	case "call_expression":
		if name := flatCallExpressionName(file, call); name != "" {
			return name
		}
		return flatCallNameAny(file, call)
	case "method_invocation":
		return javaMethodInvocationName(file, call)
	default:
		return ""
	}
}

func databaseCallReceiverName(file *scanner.File, call uint32) string {
	if file == nil || call == 0 {
		return ""
	}
	switch file.FlatType(call) {
	case "call_expression":
		return flatReceiverNameFromCall(file, call)
	case "method_invocation":
		return wrongViewCastCallReceiverName(file, call)
	default:
		return ""
	}
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

func (r *RecyclerAdapterWithoutDiffUtilRule) Confidence() float64 { return api.ConfidenceMedium }

// RecyclerAdapterStableIDsDefaultRule detects RecyclerView.Adapter subclasses
// that don't call setHasStableIds(true) and don't extend ListAdapter.
type RecyclerAdapterStableIDsDefaultRule struct {
	FlatDispatchBase
	BaseRule
}

func (r *RecyclerAdapterStableIDsDefaultRule) Confidence() float64 { return api.ConfidenceMedium }

// LazyColumnInsideColumnRule detects LazyColumn nested inside a Column with
// verticalScroll modifier.
type LazyColumnInsideColumnRule struct {
	FlatDispatchBase
	BaseRule
}

func (r *LazyColumnInsideColumnRule) Confidence() float64 { return api.ConfidenceMedium }

// RecyclerViewInLazyColumnRule detects AndroidView wrapping a RecyclerView
// inside a LazyColumn/LazyRow.
type RecyclerViewInLazyColumnRule struct {
	FlatDispatchBase
	BaseRule
}

func (r *RecyclerViewInLazyColumnRule) Confidence() float64 { return api.ConfidenceMedium }

// ---------------------------------------------------------------------------
// Batch 5: Image Loading rules
// ---------------------------------------------------------------------------

// ImageLoadedAtFullSizeInListRule detects Glide/Coil image loading without
// size constraints in list item contexts.
type ImageLoadedAtFullSizeInListRule struct {
	FlatDispatchBase
	BaseRule
}

func (r *ImageLoadedAtFullSizeInListRule) Confidence() float64 { return api.ConfidenceMedium }

// ImageLoaderNoMemoryCacheRule detects image loaders configured to skip
// the memory cache.
type ImageLoaderNoMemoryCacheRule struct {
	FlatDispatchBase
	BaseRule
}

func (r *ImageLoaderNoMemoryCacheRule) Confidence() float64 { return api.ConfidenceMedium }

// ---------------------------------------------------------------------------
// Batch 6: Compose rules
// ---------------------------------------------------------------------------

// ComposePainterResourceInLoopRule detects painterResource() inside
// forEach/items lambda bodies.
type ComposePainterResourceInLoopRule struct {
	FlatDispatchBase
	BaseRule
}

func (r *ComposePainterResourceInLoopRule) Confidence() float64 { return api.ConfidenceMedium }

// ComposeRememberInListRule detects remember{} inside items{} lambda
// without a key argument — causes recomputation on list reordering.
type ComposeRememberInListRule struct {
	FlatDispatchBase
	BaseRule
}

func (r *ComposeRememberInListRule) Confidence() float64 { return api.ConfidenceMedium }

// ---------------------------------------------------------------------------
// Batch 7: WorkManager rules
// ---------------------------------------------------------------------------

// PeriodicWorkRequestLessThan15MinRule detects PeriodicWorkRequestBuilder
// with an interval less than 15 minutes.
type PeriodicWorkRequestLessThan15MinRule struct {
	FlatDispatchBase
	BaseRule
}

func (r *PeriodicWorkRequestLessThan15MinRule) Confidence() float64 { return api.ConfidenceMedium }

// WorkManagerNoBackoffRule detects OneTimeWorkRequestBuilder chains without
// setBackoffCriteria.
type WorkManagerNoBackoffRule struct {
	FlatDispatchBase
	BaseRule
}

func (r *WorkManagerNoBackoffRule) Confidence() float64 { return api.ConfidenceMedium }

// WorkManagerUniquePolicyKeepButReplaceIntendedRule detects enqueueUniqueWork
// with ExistingWorkPolicy.KEEP where REPLACE may be intended.
type WorkManagerUniquePolicyKeepButReplaceIntendedRule struct {
	FlatDispatchBase
	BaseRule
}

func (r *WorkManagerUniquePolicyKeepButReplaceIntendedRule) Confidence() float64 {
	return api.ConfidenceMedium
}

// rememberInListCallNames are the lazy list builders whose content lambda runs once per item.
var rememberInListCallNames = map[string]bool{"items": true, "itemsIndexed": true}

// rememberInListBody returns the trailing lambda of a remember call, or 0.
func rememberInListBody(file *scanner.File, call uint32) uint32 {
	suffix, _ := file.FlatFindChild(call, "call_suffix")
	if suffix == 0 {
		return 0
	}
	if annotated, _ := file.FlatFindChild(suffix, "annotated_lambda"); annotated != 0 {
		lambda, _ := file.FlatFindChild(annotated, "lambda_literal")
		return lambda
	}
	lambda, _ := file.FlatFindChild(suffix, "lambda_literal")
	return lambda
}

// okHTTPClientToolingPath reports tooling code, which runs once per process. Scripts are skipped by the
// resource-cost script scope.
func okHTTPClientToolingPath(path string) bool {
	slash := "/" + strings.TrimPrefix(filepath.ToSlash(path), "/")
	return strings.Contains(slash, "/tooling/") || strings.Contains(slash, "/scripts/")
}

// okHTTPClientFeedsOneShotInitializer reports whether the client is built inside a lazy block, or in a function
// that the file only calls from val initializers or lazy blocks, so it runs once.
func okHTTPClientFeedsOneShotInitializer(file *scanner.File, idx, fn uint32) bool {
	for cur, ok := file.FlatParent(idx); ok && cur != fn; cur, ok = file.FlatParent(cur) {
		if file.FlatType(cur) == "call_expression" && flatCallNameAny(file, cur) == "lazy" {
			return true
		}
	}
	if file.FlatType(fn) != "function_declaration" {
		return false
	}
	name := flatFunctionName(file, fn)
	if name == "" {
		return false
	}
	callSites, oneShot := 0, true
	file.FlatWalkNodes(0, "call_expression", func(call uint32) {
		if !oneShot || flatCallNameAny(file, call) != name || flatNodeHasAncestor(file, call, fn) {
			return
		}
		callSites++
		if !callIsInOneShotInitializer(file, call) {
			oneShot = false
		}
	})
	return oneShot && callSites > 0
}

func callIsInOneShotInitializer(file *scanner.File, call uint32) bool {
	for cur, ok := file.FlatParent(call); ok; cur, ok = file.FlatParent(cur) {
		switch file.FlatType(cur) {
		case "function_declaration", "while_statement", "for_statement", "do_while_statement":
			return false
		case "call_expression":
			if flatCallNameAny(file, cur) == "lazy" {
				return true
			}
		case "property_declaration":
			// Local vals sit in function bodies that may run repeatedly.
			parent, _ := file.FlatParent(cur)
			if pt := file.FlatType(parent); !propertyDeclarationIsVar(file, cur) && (pt == "class_body" || pt == "source_file") {
				return true
			}
		}
	}
	return false
}

// okHTTPExecuteIsCancellableBridge reports execute() inside suspendCancellableCoroutine that cancels the call on coroutine cancellation.
func okHTTPExecuteIsCancellableBridge(file *scanner.File, idx uint32) bool {
	for cur, ok := file.FlatParent(idx); ok; cur, ok = file.FlatParent(cur) {
		if file.FlatType(cur) == "call_expression" && flatCallNameAny(file, cur) == "suspendCancellableCoroutine" {
			text := file.FlatNodeText(cur)
			return strings.Contains(text, "invokeOnCancellation") && strings.Contains(text, ".cancel()")
		}
	}
	return false
}

// functionKdocRequiresIODispatcher reports a KDoc that tells callers to run the function on an IO dispatcher.
func functionKdocRequiresIODispatcher(file *scanner.File, fn uint32) bool {
	prev, ok := file.FlatPrevSibling(fn)
	if !ok || file.FlatType(prev) != "multiline_comment" || !flatIsKDoc(file, prev) {
		return false
	}
	text := strings.ToLower(file.FlatNodeText(prev))
	return strings.Contains(text, "dispatchers.io") || strings.Contains(text, "io dispatcher") ||
		strings.Contains(text, "background thread") || strings.Contains(text, "iodispatcher")
}

// cursorCallReceiverNode returns the receiver of a Kotlin call or Java method invocation, or 0 when it has none.
func cursorCallReceiverNode(file *scanner.File, call uint32) uint32 {
	if file.FlatType(call) != "method_invocation" {
		return astReceiverNodeFromCall(file, call)
	}
	first := file.FlatFirstChild(call)
	if next := file.FlatNextSib(first); first != 0 && next != 0 && file.FlatType(next) == "." {
		return first
	}
	return 0
}

// cursorLoopLocalNames returns names declared inside a cursor loop: locals, lambda parameters and for variables.
func cursorLoopLocalNames(file *scanner.File, body uint32) map[string]bool {
	names := map[string]bool{}
	file.FlatWalkNodes(body, "property_declaration", func(prop uint32) {
		if n := composePropertyNameFlat(file, prop); n != "" {
			names[n] = true
		}
	})
	file.FlatWalkNodes(body, "for_statement", func(forStmt uint32) {
		if decl, ok := file.FlatFindChild(forStmt, "variable_declaration"); ok {
			if ident, ok := file.FlatFindChild(decl, "simple_identifier"); ok {
				names[file.FlatNodeText(ident)] = true
			}
		}
	})
	file.FlatWalkNodes(body, "lambda_literal", func(lambda uint32) {
		params := composeLambdaParameterNamesFlat(file, lambda)
		if len(params) == 0 {
			names["it"] = true
		}
		for _, p := range params {
			names[p] = true
		}
	})
	return names
}

// cursorColumnArgIsLoopInvariant reports whether the column-name argument of a getColumnIndex call does not
// depend on a value declared inside the loop, so the lookup can be hoisted.
func cursorColumnArgIsLoopInvariant(file *scanner.File, call uint32, loopLocals map[string]bool) bool {
	args := flatCallKeyArguments(file, call)
	if args == 0 {
		args, _ = file.FlatFindChild(call, "argument_list")
	}
	if args == 0 {
		return true
	}
	return !resourceCostSubtreeUsesName(file, args, loopLocals)
}

// bufferedReadFillsLargeBuffer reports whether a FileInputStream.read call fills a buffer known to hold 8 KiB
// or more, which already moves data in large chunks. Reads into buffers of unknown size are not exempt.
func bufferedReadFillsLargeBuffer(file *scanner.File, call uint32) bool {
	first := flatPositionalValueArgument(file, flatCallKeyArguments(file, call), 0)
	expr := flatValueArgumentExpression(file, first)
	if expr == 0 {
		return false
	}
	size, known := byteArraySizeLiteral(file, expr)
	if !known && file.FlatType(expr) == "simple_identifier" {
		name := file.FlatNodeText(expr)
		if fn, ok := flatEnclosingCallable(file, call); ok {
			file.FlatWalkNodes(fn, "property_declaration", func(prop uint32) {
				if known || composePropertyNameFlat(file, prop) != name || declarationIsVarFlat(file, prop) {
					return
				}
				if init := propertyInitializerExpression(file, prop); init != 0 {
					size, known = byteArraySizeLiteral(file, init)
				}
			})
		}
	}
	return known && size >= 8192
}

// byteArraySizeLiteral returns N for a `ByteArray(N)` expression with an integer literal size.
func byteArraySizeLiteral(file *scanner.File, expr uint32) (int, bool) {
	if file.FlatType(expr) != "call_expression" || flatCallExpressionName(file, expr) != "ByteArray" {
		return 0, false
	}
	args := flatCallKeyArguments(file, expr)
	arg := flatPositionalValueArgument(file, args, 0)
	size := flatValueArgumentExpression(file, arg)
	if size == 0 {
		return 0, false
	}
	n, err := strconv.Atoi(strings.ReplaceAll(strings.TrimSpace(file.FlatNodeText(size)), "_", ""))
	return n, err == nil
}

const workManagerMinPeriodicMillis = 15 * 60 * 1000

var workManagerUnitMillis = map[string]int64{
	"NANOSECONDS": 0, "MICROSECONDS": 0, "MILLISECONDS": 1, "SECONDS": 1000,
	"MINUTES": 60 * 1000, "HOURS": 60 * 60 * 1000, "DAYS": 24 * 60 * 60 * 1000,
}

var workManagerDurationFactories = map[string]int64{
	"ofMillis": 1, "ofSeconds": 1000, "ofMinutes": 60 * 1000, "ofHours": 60 * 60 * 1000, "ofDays": 24 * 60 * 60 * 1000,
}

// workManagerIntervalMillis evaluates the repeat interval of a PeriodicWorkRequest call when it is a literal
// amount with a TimeUnit, or a Duration.ofX(literal) argument.
func workManagerIntervalMillis(file *scanner.File, args uint32) (int64, bool) {
	first := flatPositionalValueArgument(file, args, 0)
	expr := flatValueArgumentExpression(file, first)
	if expr == 0 {
		return 0, false
	}
	parse := func(node uint32) (int64, bool) {
		text := strings.TrimRight(strings.ReplaceAll(strings.TrimSpace(file.FlatNodeText(node)), "_", ""), "lL")
		n, err := strconv.ParseInt(text, 10, 64)
		return n, err == nil
	}
	if file.FlatType(expr) == "call_expression" {
		factor, ok := workManagerDurationFactories[flatCallExpressionName(file, expr)]
		if !ok {
			return 0, false
		}
		inner := flatPositionalValueArgument(file, flatCallKeyArguments(file, expr), 0)
		amount, ok := parse(flatValueArgumentExpression(file, inner))
		return amount * factor, ok
	}
	amount, ok := parse(expr)
	if !ok {
		return 0, false
	}
	unit := flatPositionalValueArgument(file, args, 1)
	if unit == 0 {
		unit = flatNamedValueArgument(file, args, "repeatIntervalTimeUnit")
	}
	for name, factor := range workManagerUnitMillis {
		if callArgHasReference(file, unit, name) {
			if factor == 0 {
				return 0, true
			}
			return amount * factor, true
		}
	}
	return 0, false
}

func workManagerIntervalLabel(millis int64) string {
	switch {
	case millis%(60*1000) == 0:
		return strconv.FormatInt(millis/(60*1000), 10) + " minutes"
	case millis%1000 == 0:
		return strconv.FormatInt(millis/1000, 10) + " seconds"
	}
	return strconv.FormatInt(millis, 10) + " ms"
}
