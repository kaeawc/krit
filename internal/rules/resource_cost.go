package rules

import (
	"bytes"
	"strings"

	"github.com/kaeawc/krit/internal/javafacts"
	api "github.com/kaeawc/krit/internal/rules/api"
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
	return bytes.Contains(file.Content, []byte(fqn))
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
