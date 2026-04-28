package rules

import (
	"fmt"
	"strings"
	"sync"

	v2 "github.com/kaeawc/krit/internal/rules/v2"
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

// CursorLoopWithColumnIndexInLoopRule detects getColumnIndex() calls inside
// cursor.moveToNext() while loops.
type CursorLoopWithColumnIndexInLoopRule struct {
	FlatDispatchBase
	BaseRule
}

func (r *CursorLoopWithColumnIndexInLoopRule) Confidence() float64 { return 0.75 }

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

// OkHttpCallExecuteSyncRule detects Call.execute() inside suspend functions
// where enqueue() with a callback should be used instead.
type OkHttpCallExecuteSyncRule struct {
	FlatDispatchBase
	BaseRule
}

func (r *OkHttpCallExecuteSyncRule) Confidence() float64 { return 0.75 }

func okHTTPExecuteCallLooksBlocking(file *scanner.File, idx, fn uint32) bool {
	navText := file.FlatNodeText(idx)
	if navExpr, _ := flatCallExpressionParts(file, idx); navExpr != 0 {
		navText = file.FlatNodeText(navExpr)
	}
	if strings.Contains(navText, "newCall") || strings.Contains(navText, "okhttp3.Call") {
		return true
	}
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

type sourceImportMentionKey struct {
	file *scanner.File
	fqn  string
}

var sourceImportMentionCache sync.Map

func sourceImportsOrMentions(file *scanner.File, fqn string) bool {
	if file == nil {
		return false
	}
	key := sourceImportMentionKey{file: file, fqn: fqn}
	if cached, ok := sourceImportMentionCache.Load(key); ok {
		return cached.(bool)
	}
	text := string(file.Content)
	result := strings.Contains(text, "import "+fqn) || strings.Contains(text, fqn)
	sourceImportMentionCache.Store(key, result)
	return result
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

func retrofitCreateLooksReal(file *scanner.File, idx uint32, nodeText string) bool {
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

func (r *RetrofitCreateInHotPathRule) Confidence() float64 { return 0.75 }

// HttpClientNotReusedRule detects Java HttpClient.newHttpClient() in function
// bodies without caching.
type HttpClientNotReusedRule struct {
	FlatDispatchBase
	BaseRule
}

func (r *HttpClientNotReusedRule) Confidence() float64 { return 0.75 }

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

var sqliteQueryMethods = map[string]bool{
	"rawQuery": true,
	"query":    true,
	"execSQL":  true,
}

var sqlDelightQueryExecutionMethods = map[string]bool{
	"executeAsList":      true,
	"executeAsOne":       true,
	"executeAsOneOrNull": true,
	"executeAsOptional":  true,
	"executeAsCursor":    true,
}

var databaseMainThreadFunctionNames = map[string]bool{
	"afterTextChanged":           true,
	"beforeTextChanged":          true,
	"onActivityCreated":          true,
	"onActivityResult":           true,
	"onBindViewHolder":           true,
	"onCheckedChanged":           true,
	"onClick":                    true,
	"onContextItemSelected":      true,
	"onCreate":                   true,
	"onCreateOptionsMenu":        true,
	"onCreateView":               true,
	"onDestroy":                  true,
	"onDestroyView":              true,
	"onItemClick":                true,
	"onItemLongClick":            true,
	"onLongClick":                true,
	"onOptionsItemSelected":      true,
	"onPause":                    true,
	"onRequestPermissionsResult": true,
	"onResume":                   true,
	"onStart":                    true,
	"onStop":                     true,
	"onTextChanged":              true,
	"onViewCreated":              true,
}

var databaseMainThreadLambdaCalls = map[string]bool{
	"addTextChangedListener":     true,
	"doAfterTextChanged":         true,
	"doBeforeTextChanged":        true,
	"doOnClick":                  true,
	"doOnTextChanged":            true,
	"setOnCheckedChangeListener": true,
	"setOnClickListener":         true,
	"setOnEditorActionListener":  true,
	"setOnItemClickListener":     true,
	"setOnItemLongClickListener": true,
	"setOnLongClickListener":     true,
	"setOnMenuItemClickListener": true,
}

var databaseMainThreadOwnerHints = []string{
	"Activity",
	"Adapter",
	"Dialog",
	"Fragment",
	"LifecycleEventObserver",
	"LifecycleObserver",
	"DefaultLifecycleObserver",
	"Service",
	"View",
	"ViewHolder",
	"TextWatcher",
	"OnClickListener",
	"OnItemClickListener",
	"OnItemLongClickListener",
	"OnCheckedChangeListener",
}

func databaseQueryCallLooksSQLite(file *scanner.File, idx uint32, name string) bool {
	if file == nil || idx == 0 {
		return false
	}
	callText := strings.ToLower(file.FlatNodeText(idx))
	if strings.Contains(callText, "readabledatabase") ||
		strings.Contains(callText, "writabledatabase") ||
		strings.Contains(callText, "sqlitedatabase") ||
		strings.Contains(callText, "database.") ||
		strings.Contains(callText, "db.") {
		return true
	}
	receiver := strings.ToLower(databaseCallReceiverName(file, idx))
	if receiver == "" {
		return false
	}
	if strings.Contains(receiver, "database") || strings.Contains(receiver, "sqlite") ||
		receiver == "db" || strings.HasSuffix(receiver, "db") {
		return true
	}
	// rawQuery and execSQL are SQLite-specific enough to accept common short
	// receiver aliases; query is too generic and must pass the database check.
	if name == "rawQuery" || name == "execSQL" {
		return receiver == "readable" || receiver == "writable"
	}
	return false
}

func databaseQueryHasMainThreadEvidence(file *scanner.File, idx uint32, fn uint32) bool {
	if file == nil || idx == 0 || fn == 0 {
		return false
	}
	if databaseQueryHasMainThreadAnnotation(file, fn) {
		return true
	}
	if databaseQueryInsideMainDispatcher(file, idx) {
		return true
	}
	if databaseQueryInsideMainThreadListener(file, idx) {
		return true
	}
	fnName := databaseFunctionName(file, fn)
	if !databaseMainThreadFunctionNames[fnName] {
		return false
	}
	if databaseQueryIsSQLiteLifecycleCallback(file, fn, fnName) {
		return false
	}
	return databaseQueryEnclosingClassLooksMainThread(file, fn)
}

func databaseQueryHasMainThreadAnnotation(file *scanner.File, fn uint32) bool {
	if hasAnnotationFlat(file, fn, "MainThread") || hasAnnotationFlat(file, fn, "UiThread") {
		return true
	}
	text := file.FlatNodeText(fn)
	if bodyIdx := strings.Index(text, "{"); bodyIdx >= 0 {
		text = text[:bodyIdx]
	}
	return strings.Contains(text, "MainThread") || strings.Contains(text, "UiThread")
}

func databaseQueryInsideMainDispatcher(file *scanner.File, idx uint32) bool {
	for current, ok := file.FlatParent(idx); ok; current, ok = file.FlatParent(current) {
		switch file.FlatType(current) {
		case "call_expression", "lambda_literal", "method_invocation", "lambda_expression":
			text := file.FlatNodeText(current)
			if strings.Contains(text, "Dispatchers.Main") || strings.Contains(text, "Main.immediate") {
				return true
			}
		case "function_declaration", "method_declaration", "source_file":
			return false
		}
	}
	return false
}

func databaseQueryInsideMainThreadListener(file *scanner.File, idx uint32) bool {
	for current, ok := file.FlatParent(idx); ok; current, ok = file.FlatParent(current) {
		switch file.FlatType(current) {
		case "call_expression", "method_invocation", "lambda_literal", "lambda_expression":
			if databaseMainThreadLambdaCalls[databaseCallName(file, current)] {
				return true
			}
			text := file.FlatNodeText(current)
			for name := range databaseMainThreadLambdaCalls {
				if strings.Contains(text, name+"(") || strings.Contains(text, name+" {") {
					return true
				}
			}
		case "function_declaration", "method_declaration", "source_file":
			return false
		}
	}
	return false
}

func databaseQueryIsSQLiteLifecycleCallback(file *scanner.File, fn uint32, fnName string) bool {
	switch fnName {
	case "onConfigure", "onCreate", "onDowngrade", "onOpen", "onUpgrade":
	default:
		return false
	}
	text := file.FlatNodeText(fn)
	if bodyIdx := strings.Index(text, "{"); bodyIdx >= 0 {
		text = text[:bodyIdx]
	}
	return strings.Contains(text, "SQLiteDatabase") || strings.Contains(text, "SupportSQLiteDatabase")
}

func databaseQueryEnclosingClassLooksMainThread(file *scanner.File, fn uint32) bool {
	for current, ok := file.FlatParent(fn); ok; current, ok = file.FlatParent(current) {
		t := file.FlatType(current)
		if t != "class_declaration" && t != "object_declaration" {
			continue
		}
		header := file.FlatNodeText(current)
		if bodyIdx := strings.Index(header, "{"); bodyIdx >= 0 {
			header = header[:bodyIdx]
		}
		supertypes := databaseClassSupertypeText(file, current, header)
		if supertypes == "" {
			continue
		}
		for _, hint := range databaseMainThreadOwnerHints {
			if strings.Contains(supertypes, hint) {
				return true
			}
		}
	}
	return false
}

func databaseClassSupertypeText(file *scanner.File, classNode uint32, header string) string {
	name := databaseClassName(file, classNode)
	if name != "" {
		if nameIdx := strings.Index(header, name); nameIdx >= 0 {
			return header[nameIdx+len(name):]
		}
	}
	return header
}

func databaseClassName(file *scanner.File, classNode uint32) string {
	if name := extractIdentifierFlat(file, classNode); name != "" {
		return name
	}
	for child := file.FlatFirstChild(classNode); child != 0; child = file.FlatNextSib(child) {
		if file.FlatType(child) == "identifier" {
			return file.FlatNodeText(child)
		}
	}
	return ""
}

type databaseFunctionSummary struct {
	id                 string
	file               *scanner.File
	idx                uint32
	name               string
	owner              string
	databaseTableOwner bool
	dbCalls            []databaseCallSummary
	calls              []databaseCallSummary
	reachesDB          bool
}

type databaseCallSummary struct {
	file     *scanner.File
	idx      uint32
	name     string
	display  string
	targetID string
	deferred bool
}

func (r *DatabaseQueryOnMainThreadRule) checkParsedFiles(ctx *v2.Context) {
	if ctx == nil || len(ctx.ParsedFiles) == 0 {
		return
	}
	roomOps := databaseCollectBlockingRoomDaoOperations(ctx.ParsedFiles)
	functions := databaseCollectFunctionSummaries(ctx.ParsedFiles, roomOps)
	databaseResolveFunctionCalls(functions)
	databasePropagateBlockingDB(functions)
	signalOps := databaseCollectSignalDatabaseOperations(functions)
	if len(signalOps) > 0 {
		databaseAugmentFunctionSummariesWithSignalCalls(functions, signalOps)
	}
	databasePropagateBlockingDB(functions)

	for _, fn := range functions {
		for _, call := range fn.dbCalls {
			if !databaseQueryHasMainThreadEvidence(call.file, call.idx, fn.idx) {
				continue
			}
			ctx.Emit(r.Finding(
				call.file,
				call.file.FlatRow(call.idx)+1,
				call.file.FlatCol(call.idx)+1,
				databaseDirectMainThreadMessage(call),
			))
		}
		for _, call := range fn.calls {
			target := functions[call.targetID]
			if target == nil || !target.reachesDB {
				continue
			}
			if databaseQueryInsideBackgroundBoundary(call.file, call.idx) {
				continue
			}
			if !databaseQueryHasMainThreadEvidence(call.file, call.idx, fn.idx) {
				continue
			}
			ctx.Emit(r.Finding(
				call.file,
				call.file.FlatRow(call.idx)+1,
				call.file.FlatCol(call.idx)+1,
				databasePropagatedMainThreadMessage(functions, call, target),
			))
		}
	}
}

func databaseDirectMainThreadMessage(call databaseCallSummary) string {
	return fmt.Sprintf("%s runs on a main-thread path. Move this database call off the UI thread, for example with Dispatchers.IO, Rx subscribeOn(Schedulers.io()), or a background executor.", call.display)
}

func databasePropagatedMainThreadMessage(functions map[string]*databaseFunctionSummary, call databaseCallSummary, target *databaseFunctionSummary) string {
	targetDetail := databaseBlockingTargetDescription(functions, target, make(map[string]bool))
	return fmt.Sprintf("Call to %s() runs on a main-thread path and reaches %s. Move the database work off the UI thread, then resume lifecycle/UI work on the main thread.", call.name, targetDetail)
}

func databaseBlockingTargetDescription(functions map[string]*databaseFunctionSummary, fn *databaseFunctionSummary, seen map[string]bool) string {
	if fn == nil || seen[fn.id] {
		return "SQLiteDatabase work"
	}
	seen[fn.id] = true
	for _, call := range fn.dbCalls {
		if !call.deferred && call.display != "" {
			return call.display
		}
	}
	for _, call := range fn.calls {
		if call.deferred {
			continue
		}
		target := functions[call.targetID]
		if target == nil || !target.reachesDB {
			continue
		}
		detail := databaseBlockingTargetDescription(functions, target, seen)
		if detail == "" {
			return fmt.Sprintf("%s()", call.name)
		}
		return fmt.Sprintf("%s -> %s", databaseCallSummaryDisplay(call), detail)
	}
	return "SQLiteDatabase work"
}

func databaseCallSummaryDisplay(call databaseCallSummary) string {
	if call.display != "" {
		return call.display
	}
	return fmt.Sprintf("%s()", call.name)
}

func databaseCollectBlockingRoomDaoOperations(files []*scanner.File) map[string]bool {
	counts := make(map[string]int)
	for _, file := range files {
		if file == nil || file.FlatTree == nil {
			continue
		}
		file.FlatWalkAllNodes(0, func(idx uint32) {
			if file.FlatType(idx) != "class_declaration" || !hasAnnotationFlat(file, idx, "Dao") {
				return
			}
			body, _ := file.FlatFindChild(idx, "class_body")
			if body == 0 {
				return
			}
			for child := file.FlatFirstChild(body); child != 0; child = file.FlatNextSib(child) {
				if file.FlatType(child) != "function_declaration" || !daoFunctionHasAllowedAnnotationFlat(file, child) {
					continue
				}
				if file.FlatHasModifier(child, "suspend") || databaseFunctionReturnsAsyncRoomType(file, child) {
					continue
				}
				if name := flatFunctionName(file, child); name != "" {
					counts[name]++
				}
			}
		})
	}
	ops := make(map[string]bool, len(counts))
	for name, count := range counts {
		if count == 1 {
			ops[name] = true
		}
	}
	return ops
}

func databaseFunctionReturnsAsyncRoomType(file *scanner.File, fn uint32) bool {
	text := file.FlatNodeText(fn)
	if bodyIdx := strings.Index(text, "{"); bodyIdx >= 0 {
		text = text[:bodyIdx]
	}
	return strings.Contains(text, "Flow<") ||
		strings.Contains(text, "LiveData<") ||
		strings.Contains(text, "PagingSource<") ||
		strings.Contains(text, "DataSource.Factory<")
}

func databaseCollectFunctionSummaries(files []*scanner.File, roomOps map[string]bool) map[string]*databaseFunctionSummary {
	functions := make(map[string]*databaseFunctionSummary)
	for _, file := range files {
		if file == nil || file.FlatTree == nil {
			continue
		}
		file.FlatWalkAllNodes(0, func(idx uint32) {
			if !databaseIsFunctionDeclaration(file, idx) {
				return
			}
			name := databaseFunctionName(file, idx)
			if name == "" {
				return
			}
			fn := &databaseFunctionSummary{
				id:    databaseFunctionID(file, idx),
				file:  file,
				idx:   idx,
				name:  name,
				owner: databaseFunctionOwner(file, idx),
			}
			fn.databaseTableOwner = databaseFunctionOwnerLooksSignalTable(file, idx, fn.owner)
			databaseCollectFunctionBody(file, fn, roomOps)
			if databaseHasImmediateDBCall(fn.dbCalls) {
				fn.reachesDB = true
			}
			functions[fn.id] = fn
		})
	}
	return functions
}

func databaseHasImmediateDBCall(calls []databaseCallSummary) bool {
	for _, call := range calls {
		if !call.deferred {
			return true
		}
	}
	return false
}

func databaseCollectFunctionBody(file *scanner.File, fn *databaseFunctionSummary, roomOps map[string]bool) {
	file.FlatWalkAllNodes(fn.idx, func(idx uint32) {
		if idx == fn.idx || !databaseIsCallExpression(file, idx) {
			return
		}
		if databaseCallInsideNestedFunctionDeclaration(file, fn.idx, idx) {
			return
		}
		name := databaseCallName(file, idx)
		if name == "" {
			return
		}
		deferred := databaseCallInsideDeferredCallback(file, fn.idx, idx)
		if sqliteQueryMethods[name] {
			if !strings.Contains(file.FlatNodeText(idx), name+"(") {
				return
			}
			if !databaseQueryCallLooksSQLite(file, idx, name) {
				return
			}
			if file.FlatHasModifier(fn.idx, "suspend") || databaseQueryInsideBackgroundBoundary(file, idx) {
				return
			}
			if databaseQueryIsSQLiteLifecycleCallback(file, fn.idx, databaseFunctionName(file, fn.idx)) {
				return
			}
			fn.dbCalls = append(fn.dbCalls, databaseCallSummary{file: file, idx: idx, name: name, display: fmt.Sprintf("SQLiteDatabase.%s()", name), deferred: deferred})
			return
		}
		if fn.databaseTableOwner && databaseSignalLocalDatabaseCallLooksBlocking(file, idx, name) {
			if databaseQueryInsideBackgroundBoundary(file, idx) {
				return
			}
			fn.dbCalls = append(fn.dbCalls, databaseCallSummary{file: file, idx: idx, name: name, display: fmt.Sprintf("Signal database %s()", name), deferred: deferred})
			return
		}
		if databaseRoomDaoCallLooksBlocking(file, idx, name, roomOps) {
			if databaseQueryInsideBackgroundBoundary(file, idx) {
				return
			}
			fn.dbCalls = append(fn.dbCalls, databaseCallSummary{file: file, idx: idx, name: name, display: fmt.Sprintf("Room DAO %s()", name), deferred: deferred})
			return
		}
		if databaseSQLDelightCallLooksBlocking(file, idx, name) {
			if databaseQueryInsideBackgroundBoundary(file, idx) {
				return
			}
			fn.dbCalls = append(fn.dbCalls, databaseCallSummary{file: file, idx: idx, name: name, display: fmt.Sprintf("SQLDelight %s()", name), deferred: deferred})
			return
		}
		if receiver := databaseCallReceiverName(file, idx); receiver != "" && receiver != "this" {
			return
		}
		fn.calls = append(fn.calls, databaseCallSummary{file: file, idx: idx, name: name, display: databaseCompactCallDisplay(file, idx, fmt.Sprintf("%s()", name)), deferred: deferred})
	})
}

func databaseCollectSignalDatabaseOperations(functions map[string]*databaseFunctionSummary) map[string]bool {
	counts := make(map[string]int)
	for _, fn := range functions {
		if fn == nil || !fn.databaseTableOwner || !fn.reachesDB || fn.name == "" {
			continue
		}
		counts[fn.name]++
	}
	ops := make(map[string]bool, len(counts))
	for name := range counts {
		ops[name] = true
	}
	return ops
}

func databaseAugmentFunctionSummariesWithSignalCalls(functions map[string]*databaseFunctionSummary, signalOps map[string]bool) {
	for _, fn := range functions {
		if fn == nil || fn.file == nil || fn.file.FlatTree == nil {
			continue
		}
		fn.file.FlatWalkAllNodes(fn.idx, func(idx uint32) {
			if idx == fn.idx || !databaseIsCallExpression(fn.file, idx) {
				return
			}
			if databaseCallInsideNestedFunctionDeclaration(fn.file, fn.idx, idx) {
				return
			}
			name := databaseCallName(fn.file, idx)
			if !signalOps[name] {
				return
			}
			if !databaseSignalDatabaseReceiverLooksBlocking(fn.file, idx) {
				return
			}
			if databaseQueryInsideBackgroundBoundary(fn.file, idx) {
				return
			}
			deferred := databaseCallInsideDeferredCallback(fn.file, fn.idx, idx)
			display := databaseSignalCallDisplay(fn.file, idx, name)
			fn.dbCalls = append(fn.dbCalls, databaseCallSummary{file: fn.file, idx: idx, name: name, display: display, deferred: deferred})
			if !deferred {
				fn.reachesDB = true
			}
		})
	}
}

func databaseSignalCallDisplay(file *scanner.File, idx uint32, name string) string {
	fallback := fmt.Sprintf("SignalDatabase.%s()", name)
	return databaseCompactCallDisplay(file, idx, fallback)
}

func databaseCompactCallDisplay(file *scanner.File, idx uint32, fallback string) string {
	if file == nil || idx == 0 {
		return fallback
	}
	text := strings.Join(strings.Fields(file.FlatNodeText(idx)), " ")
	if text == "" || len(text) > 140 {
		return fallback
	}
	return text
}

var signalLocalDatabaseCallNames = map[string]bool{
	"count":                     true,
	"delete":                    true,
	"deleteAll":                 true,
	"execSQL":                   true,
	"exists":                    true,
	"insert":                    true,
	"insertInto":                true,
	"insertOrThrow":             true,
	"insertWithOnConflict":      true,
	"query":                     true,
	"rawExecSQL":                true,
	"rawQuery":                  true,
	"replace":                   true,
	"replaceOrThrow":            true,
	"run":                       true,
	"select":                    true,
	"update":                    true,
	"updateAll":                 true,
	"updateWithOnConflict":      true,
	"withinTransaction":         true,
	"getSignalReadableDatabase": true,
	"getSignalWritableDatabase": true,
}

func databaseSignalLocalDatabaseCallLooksBlocking(file *scanner.File, idx uint32, name string) bool {
	if !signalLocalDatabaseCallNames[name] {
		return false
	}
	text := strings.ToLower(file.FlatNodeText(idx))
	receiver := strings.ToLower(databaseCallReceiverName(file, idx))
	return strings.Contains(text, "readabledatabase") ||
		strings.Contains(text, "writabledatabase") ||
		strings.Contains(text, "rawreadabledatabase") ||
		strings.Contains(text, "rawwritabledatabase") ||
		strings.Contains(text, "getsignalreadabledatabase") ||
		strings.Contains(text, "getsignalwritabledatabase") ||
		strings.Contains(receiver, "readabledatabase") ||
		strings.Contains(receiver, "writabledatabase")
}

func databaseSignalDatabaseReceiverLooksBlocking(file *scanner.File, idx uint32) bool {
	text := file.FlatNodeText(idx)
	if strings.Contains(text, "SignalDatabase.") || strings.Contains(text, "SignalDatabase") && strings.Contains(text, "().") {
		return true
	}
	receiver := databaseCallReceiverName(file, idx)
	if strings.Contains(receiver, "SignalDatabase") {
		return true
	}
	return false
}

func databaseRoomDaoCallLooksBlocking(file *scanner.File, idx uint32, name string, roomOps map[string]bool) bool {
	if !roomOps[name] {
		return false
	}
	receiver := strings.ToLower(databaseCallReceiverName(file, idx))
	if receiver == "" {
		return false
	}
	return receiver == "dao" || strings.Contains(receiver, "dao")
}

func databaseSQLDelightCallLooksBlocking(file *scanner.File, idx uint32, name string) bool {
	if !sqlDelightQueryExecutionMethods[name] {
		return false
	}
	text := strings.ToLower(file.FlatNodeText(idx))
	receiver := strings.ToLower(databaseCallReceiverName(file, idx))
	return strings.Contains(text, "queries") ||
		strings.Contains(text, "query") ||
		strings.Contains(text, "select") ||
		strings.Contains(receiver, "query")
}

func databaseCallInsideNestedFunctionDeclaration(file *scanner.File, root, call uint32) bool {
	for parent, ok := file.FlatParent(call); ok && parent != root; parent, ok = file.FlatParent(parent) {
		if databaseIsFunctionDeclaration(file, parent) {
			return true
		}
	}
	return false
}

func databaseCallInsideDeferredCallback(file *scanner.File, root, call uint32) bool {
	for parent, ok := file.FlatParent(call); ok && parent != root; parent, ok = file.FlatParent(parent) {
		if strings.Contains(file.FlatType(parent), "lambda") {
			return true
		}
	}
	return false
}

func databaseIsFunctionDeclaration(file *scanner.File, idx uint32) bool {
	if file == nil || idx == 0 {
		return false
	}
	switch file.FlatType(idx) {
	case "function_declaration", "method_declaration":
		return true
	default:
		return false
	}
}

func databaseIsCallExpression(file *scanner.File, idx uint32) bool {
	if file == nil || idx == 0 {
		return false
	}
	switch file.FlatType(idx) {
	case "call_expression", "method_invocation":
		return true
	default:
		return false
	}
}

func databaseFunctionName(file *scanner.File, fn uint32) string {
	if file == nil || fn == 0 {
		return ""
	}
	if file.FlatType(fn) == "function_declaration" {
		return flatFunctionName(file, fn)
	}
	for child := file.FlatFirstChild(fn); child != 0; child = file.FlatNextSib(child) {
		if file.FlatType(child) == "identifier" {
			return file.FlatNodeText(child)
		}
	}
	return ""
}

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
		return wrongViewCastCallName(file, call)
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

func databaseResolveFunctionCalls(functions map[string]*databaseFunctionSummary) {
	byOwnerName := make(map[string][]string, len(functions))
	for id, fn := range functions {
		key := databaseFunctionLookupKey(fn.file.Path, fn.owner, fn.name)
		byOwnerName[key] = append(byOwnerName[key], id)
	}
	for _, fn := range functions {
		for i := range fn.calls {
			call := &fn.calls[i]
			if call.name == "" {
				continue
			}
			if ids := byOwnerName[databaseFunctionLookupKey(fn.file.Path, fn.owner, call.name)]; len(ids) == 1 && ids[0] != fn.id {
				call.targetID = ids[0]
				continue
			}
			if fn.owner != databaseTopLevelOwner {
				if ids := byOwnerName[databaseFunctionLookupKey(fn.file.Path, databaseTopLevelOwner, call.name)]; len(ids) == 1 && ids[0] != fn.id {
					call.targetID = ids[0]
				}
			}
		}
	}
}

func databasePropagateBlockingDB(functions map[string]*databaseFunctionSummary) {
	for changed := true; changed; {
		changed = false
		for _, fn := range functions {
			if fn.reachesDB {
				continue
			}
			for _, call := range fn.calls {
				target := functions[call.targetID]
				if call.deferred || target == nil || !target.reachesDB || databaseQueryInsideBackgroundBoundary(call.file, call.idx) {
					continue
				}
				fn.reachesDB = true
				changed = true
				break
			}
		}
	}
}

const databaseTopLevelOwner = "<top>"

func databaseFunctionID(file *scanner.File, fn uint32) string {
	return fmt.Sprintf("%s:%d", file.Path, file.FlatStartByte(fn))
}

func databaseFunctionLookupKey(filePath, owner, name string) string {
	return filePath + "\x00" + owner + "\x00" + name
}

func databaseFunctionOwner(file *scanner.File, fn uint32) string {
	var parts []string
	for current, ok := file.FlatParent(fn); ok; current, ok = file.FlatParent(current) {
		t := file.FlatType(current)
		if t != "class_declaration" && t != "object_declaration" {
			continue
		}
		name := extractIdentifierFlat(file, current)
		if name == "" {
			continue
		}
		parts = append(parts, name)
	}
	if len(parts) == 0 {
		return databaseTopLevelOwner
	}
	for i, j := 0, len(parts)-1; i < j; i, j = i+1, j-1 {
		parts[i], parts[j] = parts[j], parts[i]
	}
	return strings.Join(parts, ".")
}

func databaseFunctionOwnerLooksSignalTable(file *scanner.File, fn uint32, owner string) bool {
	if owner == "" || owner == databaseTopLevelOwner {
		return false
	}
	for current, ok := file.FlatParent(fn); ok; current, ok = file.FlatParent(current) {
		t := file.FlatType(current)
		if t != "class_declaration" && t != "object_declaration" {
			continue
		}
		name := extractIdentifierFlat(file, current)
		if name == "" || !strings.HasSuffix(owner, name) {
			continue
		}
		header := file.FlatNodeText(current)
		if bodyIdx := strings.Index(header, "{"); bodyIdx >= 0 {
			header = header[:bodyIdx]
		}
		return strings.Contains(header, "DatabaseTable") ||
			strings.Contains(header, "SQLiteOpenHelper") ||
			strings.Contains(header, "SignalDatabaseOpenHelper") ||
			strings.HasSuffix(name, "Table") ||
			strings.HasSuffix(name, "Tables") ||
			strings.HasSuffix(name, "Database")
	}
	return false
}

func databaseQueryInsideBackgroundBoundary(file *scanner.File, idx uint32) bool {
	enclosingText := databaseEnclosingFunctionText(file, idx)
	for current, ok := file.FlatParent(idx); ok; current, ok = file.FlatParent(current) {
		switch file.FlatType(current) {
		case "call_expression", "lambda_literal", "method_invocation", "lambda_expression":
			text := file.FlatNodeText(current)
			lower := strings.ToLower(text)
			if strings.Contains(text, "Dispatchers.IO") ||
				strings.Contains(text, "Dispatchers.Default") ||
				strings.Contains(text, "Schedulers.io()") ||
				strings.Contains(text, "Schedulers.computation()") ||
				strings.Contains(text, "subscribeOn(Schedulers.io") ||
				strings.Contains(text, "subscribeOn(Schedulers.computation") ||
				strings.Contains(text, "SimpleTask.run") ||
				strings.Contains(text, "SignalExecutors.BOUNDED.execute") ||
				strings.Contains(text, "SignalExecutors.UNBOUNDED.execute") ||
				strings.Contains(text, "SignalExecutors.SERIAL.execute") ||
				strings.Contains(text, "executeOnExecutor(SignalExecutors.") ||
				strings.Contains(lower, "iodispatcher") ||
				strings.Contains(lower, "databasedispatcher") ||
				strings.Contains(lower, "backgrounddispatcher") {
				return true
			}
			if (strings.Contains(text, "fromCallable") || strings.Contains(text, "fromAction") || strings.Contains(text, "fromRunnable")) &&
				(strings.Contains(enclosingText, "subscribeOn(Schedulers.io") || strings.Contains(enclosingText, "subscribeOn(Schedulers.computation")) {
				return true
			}
			if file.FlatType(current) == "call_expression" || file.FlatType(current) == "method_invocation" {
				name := databaseCallName(file, current)
				receiver := strings.ToLower(databaseCallReceiverName(file, current))
				if (name == "execute" || name == "submit") &&
					(strings.Contains(receiver, "executor") || strings.Contains(receiver, "background")) {
					return true
				}
			}
		case "function_declaration", "method_declaration", "source_file":
			return false
		}
	}
	return false
}

func databaseEnclosingFunctionText(file *scanner.File, idx uint32) string {
	for current, ok := file.FlatParent(idx); ok; current, ok = file.FlatParent(current) {
		if databaseIsFunctionDeclaration(file, current) {
			return file.FlatNodeText(current)
		}
	}
	return ""
}

// RoomLoadsAllWhereFirstUsedRule detects dao.getAll().first() or similar
// patterns that load an entire table for a single element.
type RoomLoadsAllWhereFirstUsedRule struct {
	FlatDispatchBase
	BaseRule
}

func (r *RoomLoadsAllWhereFirstUsedRule) Confidence() float64 { return 0.75 }

func (r *RoomLoadsAllWhereFirstUsedRule) checkParsedFiles(ctx *v2.Context) {
	if ctx == nil || len(ctx.ParsedFiles) == 0 {
		return
	}
	roomLoadAll := roomCollectLoadAllDaoOperations(ctx.ParsedFiles)
	if len(roomLoadAll) == 0 {
		return
	}
	for _, file := range ctx.ParsedFiles {
		if file == nil || file.FlatTree == nil {
			continue
		}
		file.FlatWalkAllNodes(0, func(idx uint32) {
			if file.FlatType(idx) != "call_expression" {
				return
			}
			terminal := flatCallExpressionName(file, idx)
			if !loadAllTerminalMethods[terminal] {
				return
			}
			receiverCall, receiverName := roomLoadAllReceiverCall(file, idx)
			if receiverCall == 0 || !roomLoadAll[receiverName] {
				return
			}
			if !roomLoadAllReceiverLooksDao(file, receiverCall) {
				return
			}
			ctx.Emit(r.Finding(
				file,
				file.FlatRow(idx)+1,
				file.FlatCol(idx)+1,
				fmt.Sprintf("Room DAO %s().%s() loads all rows before selecting one element. Add LIMIT 1 to the @Query, or expose a dedicated DAO method that returns a single row.", receiverName, terminal),
			))
		})
	}
}

func roomCollectLoadAllDaoOperations(files []*scanner.File) map[string]bool {
	counts := make(map[string]int)
	for _, file := range files {
		if file == nil || file.FlatTree == nil {
			continue
		}
		file.FlatWalkAllNodes(0, func(idx uint32) {
			if file.FlatType(idx) != "class_declaration" || !hasAnnotationFlat(file, idx, "Dao") {
				return
			}
			body, _ := file.FlatFindChild(idx, "class_body")
			if body == 0 {
				return
			}
			for child := file.FlatFirstChild(body); child != 0; child = file.FlatNextSib(child) {
				if file.FlatType(child) != "function_declaration" || !hasAnnotationFlat(file, child, "Query") {
					continue
				}
				name := flatFunctionName(file, child)
				if !loadAllMethods[name] {
					continue
				}
				if roomQueryHasLimit(file, child) {
					continue
				}
				counts[name]++
			}
		})
	}
	ops := make(map[string]bool, len(counts))
	for name, count := range counts {
		if count == 1 {
			ops[name] = true
		}
	}
	return ops
}

func roomQueryHasLimit(file *scanner.File, fn uint32) bool {
	text := strings.ToLower(file.FlatNodeText(fn))
	return strings.Contains(text, " limit ") ||
		strings.Contains(text, " limit\n") ||
		strings.Contains(text, " limit\t") ||
		strings.Contains(text, " limit 1") ||
		strings.Contains(text, "limit :")
}

func roomLoadAllReceiverCall(file *scanner.File, idx uint32) (uint32, string) {
	navExpr, _ := flatCallExpressionParts(file, idx)
	if navExpr == 0 {
		return 0, ""
	}
	for child := file.FlatFirstChild(navExpr); child != 0; child = file.FlatNextSib(child) {
		if file.FlatType(child) == "call_expression" {
			name := flatCallExpressionName(file, child)
			if name != "" {
				return child, name
			}
		}
	}
	return 0, ""
}

func roomLoadAllReceiverLooksDao(file *scanner.File, receiverCall uint32) bool {
	receiver := strings.ToLower(flatReceiverNameFromCall(file, receiverCall))
	text := strings.ToLower(file.FlatNodeText(receiverCall))
	return strings.Contains(receiver, "dao") || strings.Contains(text, "dao.")
}

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

// RecyclerAdapterStableIdsDefaultRule detects RecyclerView.Adapter subclasses
// that don't call setHasStableIds(true) and don't extend ListAdapter.
type RecyclerAdapterStableIdsDefaultRule struct {
	FlatDispatchBase
	BaseRule
}

func (r *RecyclerAdapterStableIdsDefaultRule) Confidence() float64 { return 0.75 }

var lazyColumnToken = []byte("LazyColumn")
var lazyRowToken = []byte("LazyRow")

// LazyColumnInsideColumnRule detects LazyColumn nested inside a Column with
// verticalScroll modifier.
type LazyColumnInsideColumnRule struct {
	FlatDispatchBase
	BaseRule
}

func (r *LazyColumnInsideColumnRule) Confidence() float64 { return 0.75 }

// RecyclerViewInLazyColumnRule detects AndroidView wrapping a RecyclerView
// inside a LazyColumn/LazyRow.
type RecyclerViewInLazyColumnRule struct {
	FlatDispatchBase
	BaseRule
}

func (r *RecyclerViewInLazyColumnRule) Confidence() float64 { return 0.75 }

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

// ImageLoaderNoMemoryCacheRule detects image loaders configured to skip
// the memory cache.
type ImageLoaderNoMemoryCacheRule struct {
	FlatDispatchBase
	BaseRule
}

func (r *ImageLoaderNoMemoryCacheRule) Confidence() float64 { return 0.75 }

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

// ComposeRememberInListRule detects remember{} inside items{} lambda
// without a key argument — causes recomputation on list reordering.
type ComposeRememberInListRule struct {
	FlatDispatchBase
	BaseRule
}

func (r *ComposeRememberInListRule) Confidence() float64 { return 0.75 }

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

// WorkManagerNoBackoffRule detects OneTimeWorkRequestBuilder chains without
// setBackoffCriteria.
type WorkManagerNoBackoffRule struct {
	FlatDispatchBase
	BaseRule
}

func (r *WorkManagerNoBackoffRule) Confidence() float64 { return 0.75 }

// WorkManagerUniquePolicyKeepButReplaceIntendedRule detects enqueueUniqueWork
// with ExistingWorkPolicy.KEEP where REPLACE may be intended.
type WorkManagerUniquePolicyKeepButReplaceIntendedRule struct {
	FlatDispatchBase
	BaseRule
}

func (r *WorkManagerUniquePolicyKeepButReplaceIntendedRule) Confidence() float64 { return 0.75 }
