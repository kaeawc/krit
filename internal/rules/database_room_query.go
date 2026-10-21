package rules

import (
	"bytes"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// RoomQueryMissingWhereForUpdateRule detects @Query("UPDATE ...") or
// @Query("DELETE ...") whose SQL text omits a WHERE clause, unless the DAO
// function name signals a deliberate full-table operation (deleteAll/clearAll).
type RoomQueryMissingWhereForUpdateRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. The check matches on
// annotation name and SQL keywords without confirming the annotated function
// is a Room DAO member.
func (r *RoomQueryMissingWhereForUpdateRule) Confidence() float64 { return api.ConfidenceHigh }

var (
	roomQueryMissingWhereSQLPattern = regexp.MustCompile(`(?is)^\s*(UPDATE|DELETE)\b`)
	roomQueryWhereClausePattern     = regexp.MustCompile(`(?i)\bWHERE\b`)
)

func roomQueryMissingWhereSQL(sql string) (string, bool) {
	trimmed := strings.TrimSpace(sql)
	m := roomQueryMissingWhereSQLPattern.FindStringSubmatch(trimmed)
	if m == nil {
		return "", false
	}
	if roomQueryWhereClausePattern.MatchString(trimmed) {
		return "", false
	}
	return strings.ToUpper(m[1]), true
}

func roomQueryAnnotationSQL(text string) string {
	openIdx := strings.Index(text, "(")
	closeIdx := strings.LastIndex(text, ")")
	if openIdx < 0 || closeIdx <= openIdx {
		return ""
	}
	inner := strings.TrimSpace(text[openIdx+1 : closeIdx])
	if eqIdx := strings.Index(inner, "="); eqIdx >= 0 {
		head := strings.TrimSpace(inner[:eqIdx])
		if head == "value" {
			inner = strings.TrimSpace(inner[eqIdx+1:])
		}
	}
	if len(inner) >= 6 && strings.HasPrefix(inner, `"""`) && strings.HasSuffix(inner, `"""`) {
		return inner[3 : len(inner)-3]
	}
	if len(inner) >= 2 && strings.HasPrefix(inner, `"`) && strings.HasSuffix(inner, `"`) {
		unquoted, err := strconv.Unquote(inner)
		if err == nil {
			return unquoted
		}
		return inner[1 : len(inner)-1]
	}
	return ""
}

func roomQueryFunctionNameAllowsFullTable(name string) bool {
	if name == "" {
		return false
	}
	lower := strings.ToLower(name)
	return strings.HasPrefix(lower, "deleteall") || strings.HasPrefix(lower, "clearall")
}

// RoomSelectStarWithoutLimitRule detects Room @Query("SELECT * ...") whose SQL
// has no LIMIT clause and whose function returns an unbounded materialised
// collection (i.e., not a Flow/SharedFlow/StateFlow/PagingSource/Observable).
type RoomSelectStarWithoutLimitRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. The check matches on
// annotation name and SQL text, plus a textual return-type check, without
// confirming the annotated function is a Room DAO member.
func (r *RoomSelectStarWithoutLimitRule) Confidence() float64 { return api.ConfidenceHigh }

var (
	roomSelectStarPattern   = regexp.MustCompile(`(?is)^\s*SELECT\s+\*`)
	roomSelectLimitPattern  = regexp.MustCompile(`(?i)\bLIMIT\b`)
	roomReturnTypeStreaming = map[string]struct{}{
		"Flow":              {},
		"SharedFlow":        {},
		"StateFlow":         {},
		"MutableSharedFlow": {},
		"MutableStateFlow":  {},
		"PagingSource":      {},
		"DataSource":        {},
		"LiveData":          {},
		"MutableLiveData":   {},
		"Observable":        {},
		"Flowable":          {},
		"Single":            {},
		"Maybe":             {},
		"Publisher":         {},
		"Cursor":            {},
	}
)

func roomSelectStarMissingLimit(sql string) bool {
	trimmed := strings.TrimSpace(sql)
	if !roomSelectStarPattern.MatchString(trimmed) {
		return false
	}
	if roomSelectLimitPattern.MatchString(trimmed) {
		return false
	}
	return true
}

// roomReturnTypeAllowsUnboundedSelect reports whether the outermost type name
// is one of the streaming/observation wrappers for which a SELECT * without
// LIMIT does not eagerly load the whole table.
func roomReturnTypeAllowsUnboundedSelect(typeText string) bool {
	t := strings.TrimSpace(typeText)
	if t == "" {
		return false
	}
	if strings.HasSuffix(t, "?") {
		t = strings.TrimSuffix(t, "?")
		t = strings.TrimSpace(t)
	}
	if idx := strings.Index(t, "<"); idx >= 0 {
		t = t[:idx]
	}
	if idx := strings.LastIndex(t, "."); idx >= 0 {
		t = t[idx+1:]
	}
	t = strings.TrimSpace(t)
	_, ok := roomReturnTypeStreaming[t]
	return ok
}

// RoomQueryWithLikeMissingEscapeRule detects @Query SQL containing
// `LIKE :param` where the bind parameter is used directly without wildcard
// embedding (`'%' || :p || '%'`) or an ESCAPE directive, suggesting the
// binding is interpreted as an exact match rather than a pattern.
type RoomQueryWithLikeMissingEscapeRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-3 (low) base confidence. The check operates on
// annotation text without confirming the function is a Room DAO member, and
// rule semantics depend on the developer's intent for the LIKE pattern.
func (r *RoomQueryWithLikeMissingEscapeRule) Confidence() float64 { return api.ConfidenceMediumLowPlus }

var (
	roomLikeBareBindPattern = regexp.MustCompile(`(?i)\bLIKE\s+:([A-Za-z_][A-Za-z0-9_]*)`)
	roomLikeEscapePattern   = regexp.MustCompile(`(?i)\bESCAPE\b`)
)

// roomQueryLikeBindMissingEscape returns the first bind parameter used as a
// bare argument to LIKE (no surrounding `||` concatenation and no ESCAPE
// directive in the SQL).
func roomQueryLikeBindMissingEscape(sql string) (string, bool) {
	if roomLikeEscapePattern.MatchString(sql) {
		return "", false
	}
	matches := roomLikeBareBindPattern.FindAllStringSubmatchIndex(sql, -1)
	for _, m := range matches {
		end := m[1]
		rest := strings.TrimLeft(sql[end:], " \t\r\n")
		if strings.HasPrefix(rest, "||") {
			continue
		}
		return sql[m[2]:m[3]], true
	}
	return "", false
}

// RoomFlowQueryWithoutDistinctRule detects callers of Room @Query DAO
// functions returning Flow<...> that omit a chained .distinctUntilChanged().
type RoomFlowQueryWithoutDistinctRule struct {
	BaseRule
}

func (r *RoomFlowQueryWithoutDistinctRule) IsFixable() bool { return false }

// Confidence reports a tier-3 (low) base confidence. DAO method names are
// matched by identifier and may collide with same-named methods on non-DAO
// receivers; callers that hold the Flow in an intermediate variable are
// not flagged.
func (r *RoomFlowQueryWithoutDistinctRule) Confidence() float64 { return api.ConfidenceMediumLow }

func roomFlowReturnTypeFlat(file *scanner.File, idx uint32) bool {
	if file == nil || idx == 0 {
		return false
	}
	userType, ok := file.FlatFindChild(idx, "user_type")
	if !ok {
		return false
	}
	typeIdent, ok := file.FlatFindChild(userType, "type_identifier")
	if !ok {
		return false
	}
	return flowTypeNames[file.FlatNodeText(typeIdent)]
}

func outermostChainCallExpression(file *scanner.File, idx uint32) uint32 {
	cur := idx
	for {
		parent, ok := file.FlatParent(cur)
		if !ok {
			return cur
		}
		var first uint32
		for c := file.FlatFirstChild(parent); c != 0; c = file.FlatNextSib(c) {
			if file.FlatIsNamed(c) {
				first = c
				break
			}
		}
		switch file.FlatType(parent) {
		case "navigation_expression":
			if first == cur {
				cur = parent
				continue
			}
		case "call_expression":
			if first == cur {
				cur = parent
				continue
			}
		}
		return cur
	}
}

func (r *RoomFlowQueryWithoutDistinctRule) check(ctx *api.Context) {
	if len(ctx.ParsedFiles) == 0 {
		return
	}
	daoFlowMethods := map[string]bool{}
	for _, file := range ctx.ParsedFiles {
		if file == nil || file.FlatTree == nil {
			continue
		}
		if !bytes.Contains(file.Content, []byte("Dao")) || !bytes.Contains(file.Content, []byte("Query")) {
			continue
		}
		file.FlatWalkNodes(0, "class_declaration", func(clsIdx uint32) {
			if !hasAnnotationFlat(file, clsIdx, "Dao") {
				return
			}
			file.FlatWalkNodes(clsIdx, "function_declaration", func(fnIdx uint32) {
				if !hasAnnotationFlat(file, fnIdx, "Query") {
					return
				}
				if !roomFlowReturnTypeFlat(file, fnIdx) {
					return
				}
				if name := extractIdentifierFlat(file, fnIdx); name != "" {
					daoFlowMethods[name] = true
				}
			})
		})
	}
	if len(daoFlowMethods) == 0 {
		return
	}
	for _, file := range ctx.ParsedFiles {
		if file == nil || file.FlatTree == nil {
			continue
		}
		file.FlatWalkNodes(0, "call_expression", func(callIdx uint32) {
			name := flatCallExpressionName(file, callIdx)
			if !daoFlowMethods[name] {
				return
			}
			parent, ok := file.FlatParent(callIdx)
			if !ok || file.FlatType(parent) != "navigation_expression" {
				return
			}
			outer := outermostChainCallExpression(file, callIdx)
			chainText := file.FlatNodeText(outer)
			if strings.Contains(chainText, "distinctUntilChanged") {
				return
			}
			ctx.Emit(r.Finding(file, file.FlatRow(callIdx)+1, file.FlatCol(callIdx)+1,
				fmt.Sprintf("Caller of Room DAO Flow function '%s' is missing .distinctUntilChanged(); Room re-emits on every write to the queried tables. Chain .distinctUntilChanged() to drop redundant updates.", name)))
		})
	}
}
