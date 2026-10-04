// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
// Recall: the same contains() call, or search function, spelled in a way
// Go's name match misses.
package unmrecall

// Go reads the callee as written, backticks included, so it misses
// `contains` in backticks, the same stdlib CharSequence.contains call as
// title.contains(query).
fun findQuoted(title: String, query: String): Boolean = <!UnicodeNormalizationMissing!>title.`contains`(query)<!>

// Go reads no call name through parentheses, so it misses `(contains)(query)`,
// the same invocation of a function-typed value named contains it reports
// as `contains(query)`.
fun findParenthesized(query: String, contains: (String) -> Boolean): Boolean = <!UnicodeNormalizationMissing!>(contains)(query)<!>

// Go misses it because it reads the function name with its backticks, and
// `` `find by title` `` does not start with find; the function is a find
// function all the same.
fun `find by title`(title: String, query: String): Boolean = <!UnicodeNormalizationMissing!>title.contains(query)<!>
