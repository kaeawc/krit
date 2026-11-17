package dev.jasonpearson.krit.fir.tests

import dev.jasonpearson.krit.fir.FirRuleCompileContext
import org.junit.jupiter.api.Test
import kotlin.test.assertEquals
import kotlin.test.assertTrue

// The Go rule skips scanner.IsTestFile files: a component declared in a test
// source is not registered in the app's manifest, and Robolectric creates
// test fakes through reflection, so a private one is fine. The checker takes
// that classification from the check request, never from its own reading of
// the path.
class InstantiatableTestFileTest {

    private fun source(pkg: String) = """
        package $pkg

        import android.content.ContentProvider
        import android.content.ContentValues
        import android.database.Cursor
        import android.net.Uri

        class Host {
            private class FakeProvider : ContentProvider() {
                override fun onCreate(): Boolean = true

                override fun query(
                    uri: Uri,
                    projection: Array<String>?,
                    selection: String?,
                    selectionArgs: Array<String>?,
                    sortOrder: String?,
                ): Cursor? = null

                override fun getType(uri: Uri): String? = null

                override fun insert(uri: Uri, values: ContentValues?): Uri? = null

                override fun delete(uri: Uri, selection: String?, selectionArgs: Array<String>?): Int = 0

                override fun update(
                    uri: Uri,
                    values: ContentValues?,
                    selection: String?,
                    selectionArgs: Array<String>?,
                ): Int = 0
            }
        }
    """.trimIndent()

    private val sources = mapOf(
        "Listed.kt" to source("listed"),
        "src/test/kotlin/Unlisted.kt" to source("unlisted"),
    )

    private fun findings(testFiles: Set<String>): Map<String, Int> {
        val result = KritFirProbe.compile(
            sources,
            FirRuleCompileContext(enabledRuleIds = setOf("Instantiatable")),
            testFiles = testFiles,
        )
        assertTrue(result.clean, result.problems())
        return result.diags.filter { it.name == "Instantiatable" }.groupingBy { it.file }.eachCount()
    }

    @Test fun requestListedTestFileIsSkipped() {
        assertEquals(mapOf("Unlisted.kt" to 1), findings(setOf("Listed.kt")))
    }

    @Test fun unlistedFilesAreCheckedWhateverTheirPath() {
        assertEquals(mapOf("Listed.kt" to 1, "Unlisted.kt" to 1), findings(emptySet()))
    }
}
