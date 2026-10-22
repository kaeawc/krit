package rules

import (
	"fmt"
	"strings"

	"github.com/kaeawc/krit/internal/librarymodel"
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// ---------------------------------------------------------------------------
// Batch 3: Database rules
// ---------------------------------------------------------------------------

// DatabaseQueryOnMainThreadRule detects SQLiteDatabase.rawQuery()/query()
// calls in non-suspend functions without withContext.
type DatabaseQueryOnMainThreadRule struct {
	FlatDispatchBase
	BaseRule
}

func (r *DatabaseQueryOnMainThreadRule) Confidence() float64 { return api.ConfidenceMedium }

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

func (r *DatabaseQueryOnMainThreadRule) checkParsedFiles(ctx *api.Context) {
	if len(ctx.ParsedFiles) == 0 {
		return
	}
	facts := librarymodel.EnsureFacts(ctx.LibraryFacts)
	dbFacts := facts.Database
	roomOps := databaseCollectBlockingRoomDaoOperations(ctx.ParsedFiles, dbFacts.Room)
	functions := databaseCollectFunctionSummaries(ctx.ParsedFiles, roomOps, dbFacts)
	databaseResolveFunctionCalls(functions)
	databasePropagateBlockingDB(functions)
	serviceOps := databaseCollectDatabaseServiceOperations(functions)
	if len(serviceOps) > 0 {
		databaseAugmentFunctionSummariesWithDatabaseServiceCalls(functions, serviceOps)
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

func databaseCollectBlockingRoomDaoOperations(files []*scanner.File, roomFacts librarymodel.RoomFacts) map[string]bool {
	counts := make(map[string]int)
	for _, file := range files {
		if file == nil || file.FlatTree == nil {
			continue
		}
		file.FlatWalkAllNodes(0, func(idx uint32) {
			if file.FlatType(idx) != "class_declaration" || !databaseClassHasRoomDaoEvidence(file, idx, roomFacts) {
				return
			}
			body, _ := file.FlatFindChild(idx, "class_body")
			if body == 0 {
				return
			}
			for child := file.FlatFirstChild(body); child != 0; child = file.FlatNextSib(child) {
				if file.FlatType(child) != "function_declaration" || !databaseFunctionHasRoomAnnotationEvidence(file, child, roomFacts.OperationAnnotations, roomFacts) {
					continue
				}
				if file.FlatHasModifier(child, "suspend") || databaseFunctionReturnsAsyncRoomType(file, child, roomFacts) {
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

func databaseFunctionReturnsAsyncRoomType(file *scanner.File, fn uint32, roomFacts librarymodel.RoomFacts) bool {
	text := file.FlatNodeText(fn)
	if bodyIdx := strings.Index(text, "{"); bodyIdx >= 0 {
		text = text[:bodyIdx]
	}
	return roomFacts.IsAsyncReturnType(text)
}

func databaseHasAnyAnnotationFlat(file *scanner.File, idx uint32, names []string) bool {
	for _, name := range names {
		if hasAnnotationFlat(file, idx, name) {
			return true
		}
	}
	return false
}

func databaseClassHasRoomDaoEvidence(file *scanner.File, idx uint32, roomFacts librarymodel.RoomFacts) bool {
	if roomFacts.Enabled && databaseHasAnyAnnotationFlat(file, idx, roomFacts.DaoAnnotations) {
		return true
	}
	return databaseHasRoomSourceEvidence(file, idx, roomFacts.DaoAnnotations)
}

func databaseFunctionHasRoomAnnotationEvidence(file *scanner.File, idx uint32, names []string, roomFacts librarymodel.RoomFacts) bool {
	if roomFacts.Enabled && databaseHasAnyAnnotationFlat(file, idx, names) {
		return true
	}
	return databaseHasRoomSourceEvidence(file, idx, names)
}

func databaseHasRoomSourceEvidence(file *scanner.File, idx uint32, annotationNames []string) bool {
	if file == nil || idx == 0 {
		return false
	}
	text := string(file.Content)
	if !strings.Contains(text, "androidx.room") {
		return false
	}
	nodeText := file.FlatNodeText(idx)
	for _, name := range annotationNames {
		if strings.Contains(nodeText, "@"+name) {
			return true
		}
	}
	return false
}
