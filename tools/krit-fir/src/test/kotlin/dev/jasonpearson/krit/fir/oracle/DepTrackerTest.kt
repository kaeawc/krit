package dev.jasonpearson.krit.fir.oracle

import org.junit.jupiter.api.Test
import kotlin.test.assertEquals
import kotlin.test.assertTrue

class DepTrackerTest {

    @Test
    fun depPathsAreDedupedPerFileInInsertionOrder() {
        val t = DepTracker()
        t.recordDepPath("/src/Leaf.kt", "/src/Base.kt")
        t.recordDepPath("/src/Leaf.kt", "/src/Helper.kt")
        t.recordDepPath("/src/Leaf.kt", "/src/Base.kt")

        assertEquals(
            listOf("/src/Base.kt", "/src/Helper.kt"),
            t.depPathsByFile["/src/Leaf.kt"]?.toList(),
        )
    }

    @Test
    fun selfReferenceIsSkipped() {
        // Mirrors krit-types' DepTracker.recordDepPath: a file is
        // never listed as its own dep. The Go-side cache layer would
        // otherwise see spurious "self-dependent" entries.
        val t = DepTracker()
        t.recordDepPath("/src/Foo.kt", "/src/Foo.kt")

        assertTrue(t.depPathsByFile["/src/Foo.kt"].isNullOrEmpty())
    }

    @Test
    fun perFileDepsFirstWinsOnDuplicateFqn() {
        // Supertype resolution can revisit the same dep class via
        // multiple paths within one file (e.g. when interface A and
        // class B both extend the same Base). First-wins matches
        // krit-types' behavior.
        val t = DepTracker()
        val first = ClassPayload(fqn = "com.acme.Base", kind = "class", visibility = "public")
        val second = ClassPayload(fqn = "com.acme.Base", kind = "interface", visibility = "internal")
        t.recordPerFileDep("/src/Leaf.kt", "com.acme.Base", first)
        t.recordPerFileDep("/src/Leaf.kt", "com.acme.Base", second)

        assertEquals(first, t.perFileDeps["/src/Leaf.kt"]?.get("com.acme.Base"))
    }

    @Test
    fun crashedFilesRoundTrip() {
        val t = DepTracker()
        t.recordCrash("/src/Broken.kt", "boom")
        assertEquals(mapOf("/src/Broken.kt" to "boom"), t.crashedFiles)
    }

    @Test
    fun isEmptyReflectsCombinedState() {
        val t = DepTracker()
        assertTrue(t.isEmpty())

        t.recordDepPath("/a", "/b")
        assertTrue(!t.isEmpty())
    }
}
