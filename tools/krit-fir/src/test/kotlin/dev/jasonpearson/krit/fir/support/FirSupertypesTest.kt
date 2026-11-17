package dev.jasonpearson.krit.fir.support

import dev.jasonpearson.krit.fir.runner.AnalysisSession
import dev.jasonpearson.krit.fir.runner.FileRef
import java.io.File
import java.nio.file.Path
import kotlin.test.Test
import kotlin.test.assertEquals
import org.junit.jupiter.api.io.TempDir

/** Runs the supertype helpers through a real K2 compile via the test-only SupertypeProbe rule. */
class FirSupertypesTest {
    @TempDir lateinit var tmp: Path

    private fun probe(source: String): Map<String, String> {
        val file = tmp.resolve("Sample.kt").toFile().apply { writeText(source) }
        val stdlib = File(kotlin.Unit::class.java.protectionDomain.codeSource.location.toURI()).absolutePath
        val result = AnalysisSession(listOf(tmp.toString()), listOf(stdlib))
            .check(1, listOf(FileRef(file.absolutePath)), setOf("SupertypeProbe"), emptyMap())
        return result.findings.filter { it.rule == "SupertypeProbe" }
            .associate { it.message.substringBefore(' ') to it.message.substringAfter(' ') }
    }

    private val sample = """
        open class Base
        interface Iface
        open class Mid : Base()
        class Leaf : Mid(), Iface
        class OnlyIface : Iface
        class Unrelated
        fun useLocal() {
            open class LocalBase
            class LocalLeaf : LocalBase()
        }
    """.trimIndent()

    @Test
    fun selfCountsForIsOrInheritsFromButNotForInheritsFrom() {
        val found = probe(sample)
        assertEquals(
            "s[isOr=true,false any=false local=false] i[isOr=true,false any=false local=false]",
            found["Base"],
        )
        assertEquals(
            "s[isOr=false,true any=false local=false] i[isOr=false,true any=false local=false]",
            found["Iface"],
        )
    }

    @Test
    fun superclassWalkFollowsChainsAndSkipsInterfacesUnlessAsked() {
        val found = probe(sample)
        assertEquals(
            "s[isOr=true,false any=true local=false] i[isOr=true,true any=true local=false]",
            found["Leaf"],
        )
        assertEquals(
            "s[isOr=false,false any=false local=false] i[isOr=false,true any=true local=false]",
            found["OnlyIface"],
        )
        assertEquals(
            "s[isOr=false,false any=false local=false] i[isOr=false,false any=false local=false]",
            found["Unrelated"],
        )
    }

    @Test
    fun localClassesAreWalkedThroughLookupTags() {
        val found = probe(sample)
        assertEquals(
            "s[isOr=false,false any=false local=true] i[isOr=false,false any=false local=true]",
            found["LocalLeaf"],
        )
    }
}
