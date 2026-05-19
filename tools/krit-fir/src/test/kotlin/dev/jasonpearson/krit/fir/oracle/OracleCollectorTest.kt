package dev.jasonpearson.krit.fir.oracle

import org.junit.jupiter.api.Test
import kotlin.test.assertEquals
import kotlin.test.assertTrue

class OracleCollectorTest {

    @Test
    fun emptyCollectorProducesEmptyResult() {
        val result = OracleCollector().toResult()
        assertEquals(emptyMap(), result.files)
        assertEquals(emptyMap(), result.dependencies)
        assertEquals(emptyMap(), result.errors)
    }

    @Test
    fun addClassEmitsBothFileEntryAndDependencyEntry() {
        // The collector mirrors krit-types' two-output shape: the
        // per-file `declarations` list AND the by-FQN `dependencies`
        // map are populated from the same class declaration. The Go
        // side uses one for per-file analysis and the other for
        // cross-file symbol resolution; both must agree on the payload.
        val c = OracleCollector()
        val payload = ClassPayload(fqn = "com.acme.Foo", kind = "class")
        c.addClass("/src/Foo.kt", payload)

        val result = c.toResult()
        assertEquals(listOf(payload), result.files["/src/Foo.kt"]?.declarations)
        assertEquals(payload, result.dependencies["com.acme.Foo"])
    }

    @Test
    fun multipleClassesInOneFileAccumulateInOrder() {
        val c = OracleCollector()
        val first = ClassPayload(fqn = "com.acme.First", kind = "class")
        val second = ClassPayload(fqn = "com.acme.Second", kind = "interface")
        c.addClass("/src/Foo.kt", first)
        c.addClass("/src/Foo.kt", second)

        val declarations = c.toResult().files["/src/Foo.kt"]?.declarations
        assertEquals(listOf(first, second), declarations)
    }

    @Test
    fun duplicateFqnAcrossFilesKeepsFirstSeenInDependencies() {
        // Two files declaring the same FQN is unusual but possible
        // during incremental analysis with stale sources. Match
        // krit-types' first-wins behavior so the Go-side resolver
        // doesn't see an unpredictable "last writer wins" race.
        val c = OracleCollector()
        val firstSeen = ClassPayload(fqn = "com.acme.Foo", kind = "class", visibility = "public")
        val secondSeen = ClassPayload(fqn = "com.acme.Foo", kind = "object", visibility = "internal")
        c.addClass("/src/A.kt", firstSeen)
        c.addClass("/src/B.kt", secondSeen)

        assertEquals(firstSeen, c.toResult().dependencies["com.acme.Foo"])
    }

    @Test
    fun packageDirectiveWithoutClassDeclarationsStillEmitsFileEntry() {
        // A Kotlin file with just `package` and top-level functions
        // (no classes) should still appear in the `files` map with
        // its package name — matches krit-types' behavior of always
        // emitting one FileResult per visited file.
        val c = OracleCollector()
        c.setPackage("/src/Helpers.kt", "com.acme.util")

        val payload = c.toResult().files["/src/Helpers.kt"]
        assertEquals("com.acme.util", payload?.packageName)
        assertEquals(emptyList(), payload?.declarations)
    }

    @Test
    fun packageNameTrackedSeparatelyFromClassEntries() {
        val c = OracleCollector()
        c.setPackage("/src/Foo.kt", "com.acme")
        c.addClass("/src/Foo.kt", ClassPayload(fqn = "com.acme.Foo", kind = "class"))

        val payload = c.toResult().files["/src/Foo.kt"]
        assertEquals("com.acme", payload?.packageName)
        assertEquals(1, payload?.declarations?.size)
    }

    @Test
    fun registryScopesActiveCollectorPerThread() {
        // Sanity check on the thread-local guard. The orchestrator
        // assumes K2 runs single-threaded per compilation; the test
        // confirms a second `begin` overwrites the first, and `end`
        // clears.
        val first = OracleCollector()
        val second = OracleCollector()
        OracleCollectorRegistry.begin(first)
        assertTrue(OracleCollectorRegistry.current() === first)
        OracleCollectorRegistry.begin(second)
        assertTrue(OracleCollectorRegistry.current() === second)
        OracleCollectorRegistry.end()
        assertTrue(OracleCollectorRegistry.current() == null)
    }
}
