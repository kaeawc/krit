package rules

import (
	"fmt"
	"strings"

	"github.com/kaeawc/krit/internal/librarymodel"
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

func databaseCollectFunctionSummaries(files []*scanner.File, roomOps map[string]bool, dbFacts librarymodel.DatabaseFacts) map[string]*databaseFunctionSummary {
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
			fn.databaseTableOwner = databaseFunctionOwnerLooksDatabaseTable(file, idx, fn.owner)
			databaseCollectFunctionBody(file, fn, roomOps, dbFacts)
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

func databaseCollectFunctionBody(file *scanner.File, fn *databaseFunctionSummary, roomOps map[string]bool, dbFacts librarymodel.DatabaseFacts) {
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
		if dbFacts.IsSQLiteBlockingMethod(name) {
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
		if fn.databaseTableOwner && databaseLocalDatabaseCallLooksBlocking(file, idx, name) {
			if databaseQueryInsideBackgroundBoundary(file, idx) {
				return
			}
			fn.dbCalls = append(fn.dbCalls, databaseCallSummary{file: file, idx: idx, name: name, display: fmt.Sprintf("local database %s()", name), deferred: deferred})
			return
		}
		if databaseRoomDaoCallLooksBlocking(file, idx, name, roomOps) {
			if databaseQueryInsideBackgroundBoundary(file, idx) {
				return
			}
			fn.dbCalls = append(fn.dbCalls, databaseCallSummary{file: file, idx: idx, name: name, display: fmt.Sprintf("Room DAO %s()", name), deferred: deferred})
			return
		}
		if databaseSQLDelightCallLooksBlocking(file, idx, name, dbFacts.SQLDelight) {
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

func databaseCollectDatabaseServiceOperations(functions map[string]*databaseFunctionSummary) map[string]bool {
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

func databaseAugmentFunctionSummariesWithDatabaseServiceCalls(functions map[string]*databaseFunctionSummary, serviceOps map[string]bool) {
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
			if !serviceOps[name] {
				return
			}
			if !databaseServiceReceiverLooksBlocking(fn.file, idx) {
				return
			}
			if databaseQueryInsideBackgroundBoundary(fn.file, idx) {
				return
			}
			deferred := databaseCallInsideDeferredCallback(fn.file, fn.idx, idx)
			display := databaseServiceCallDisplay(fn.file, idx, name)
			fn.dbCalls = append(fn.dbCalls, databaseCallSummary{file: fn.file, idx: idx, name: name, display: display, deferred: deferred})
			if !deferred {
				fn.reachesDB = true
			}
		})
	}
}

func databaseServiceCallDisplay(file *scanner.File, idx uint32, name string) string {
	fallback := fmt.Sprintf("DatabaseService.%s()", name)
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

var localDatabaseCallNames = map[string]bool{
	"count":                true,
	"delete":               true,
	"deleteAll":            true,
	"execSQL":              true,
	"exists":               true,
	"insert":               true,
	"insertInto":           true,
	"insertOrThrow":        true,
	"insertWithOnConflict": true,
	"query":                true,
	"rawExecSQL":           true,
	"rawQuery":             true,
	"replace":              true,
	"replaceOrThrow":       true,
	"run":                  true,
	"select":               true,
	"update":               true,
	"updateAll":            true,
	"updateWithOnConflict": true,
	"withinTransaction":    true,
}

func databaseLocalDatabaseCallLooksBlocking(file *scanner.File, idx uint32, name string) bool {
	lowerName := strings.ToLower(name)
	if !localDatabaseCallNames[name] &&
		!strings.Contains(lowerName, "readabledatabase") &&
		!strings.Contains(lowerName, "writabledatabase") {
		return false
	}
	text := strings.ToLower(file.FlatNodeText(idx))
	receiver := strings.ToLower(databaseCallReceiverName(file, idx))
	return strings.Contains(text, "readabledatabase") ||
		strings.Contains(text, "writabledatabase") ||
		strings.Contains(text, "rawreadabledatabase") ||
		strings.Contains(text, "rawwritabledatabase") ||
		strings.Contains(receiver, "readabledatabase") ||
		strings.Contains(receiver, "writabledatabase")
}

func databaseServiceReceiverLooksBlocking(file *scanner.File, idx uint32) bool {
	text := file.FlatNodeText(idx)
	if strings.Contains(text, "Database.") || strings.Contains(text, "Database") && strings.Contains(text, "().") {
		return true
	}
	receiver := databaseCallReceiverName(file, idx)
	return strings.Contains(receiver, "Database")
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

func databaseSQLDelightCallLooksBlocking(file *scanner.File, idx uint32, name string, facts librarymodel.SQLDelightFacts) bool {
	if !facts.IsBlockingExecutionMethod(name) {
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

func databaseFunctionOwnerLooksDatabaseTable(file *scanner.File, fn uint32, owner string) bool {
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
			strings.Contains(header, "DatabaseOpenHelper") ||
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
				(strings.Contains(text, ".execute") && strings.Contains(text, "Executor")) ||
				strings.Contains(text, "executeOnExecutor(") ||
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

func (r *RoomLoadsAllWhereFirstUsedRule) Confidence() float64 { return api.ConfidenceMedium }

func (r *RoomLoadsAllWhereFirstUsedRule) checkParsedFiles(ctx *api.Context) {
	if len(ctx.ParsedFiles) == 0 {
		return
	}
	roomFacts := librarymodel.EnsureFacts(ctx.LibraryFacts).Database.Room
	roomLoadAll := roomCollectLoadAllDaoOperations(ctx.ParsedFiles, roomFacts)
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
			if !roomFacts.IsLoadOneTerminal(terminal) {
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

func roomCollectLoadAllDaoOperations(files []*scanner.File, roomFacts librarymodel.RoomFacts) map[string]bool {
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
				if file.FlatType(child) != "function_declaration" || !databaseFunctionHasRoomAnnotationEvidence(file, child, roomFacts.QueryAnnotationNames, roomFacts) {
					continue
				}
				name := flatFunctionName(file, child)
				if !roomFacts.IsLoadAllMethod(name) {
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
