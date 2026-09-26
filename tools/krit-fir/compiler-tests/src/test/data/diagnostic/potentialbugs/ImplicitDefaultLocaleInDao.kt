// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
// Go skips every file whose path ends in Dao.kt (and DAO.kt, Table.kt,
// Tables.kt, .gradle.kts); so does FIR, on the scan path.
package test

@Suppress("DEPRECATION_ERROR")
class ImplicitDefaultLocaleInDao {
    fun lower(s: String): String = s.toLowerCase()

    fun static(value: Int): String = String.format("%d", value)

    fun instance(value: Int): String = "%d".format(value)
}
