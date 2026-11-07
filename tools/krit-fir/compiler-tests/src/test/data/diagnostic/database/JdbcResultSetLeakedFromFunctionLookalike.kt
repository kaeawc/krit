// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
// Go also reports these when the file mentions java.sql; here it does not, so
// it matches FIR. Each function below is named ResultSet (or ends in
// `.ResultSet`).
// None of them is closeable or has a close() member (for a ResultSet that does,
// see JdbcResultSetLeakedFromFunctionCloseable.kt), so the message ("callers almost always forget to
// close it ... call .use {} instead") is not true of them: there is nothing to
// close and no `.use {}` to call. No finding here.
package test

class ResultSet(val rows: List<String>)

class Report {
    class ResultSet(val total: Int)
}

typealias Rows = ResultSet

fun load(): ResultSet = ResultSet(listOf("a"))

fun loadNullable(): ResultSet? = null

fun qualified(): test.ResultSet = ResultSet(emptyList())

fun nested(): Report.ResultSet = Report.ResultSet(0)

fun aliased(): Rows = ResultSet(emptyList())
