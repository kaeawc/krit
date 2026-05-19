package dev.jasonpearson.krit.fir.runner

import dev.jasonpearson.krit.fir.oracle.ClassPayload
import dev.jasonpearson.krit.fir.oracle.MemberPayload
import dev.jasonpearson.krit.fir.oracle.ParamPayload
import org.junit.jupiter.api.Test
import org.junit.jupiter.api.io.TempDir
import java.nio.file.Path
import kotlin.test.assertEquals
import kotlin.test.assertNotNull
import kotlin.test.assertTrue

/**
 * End-to-end coverage for the [`OracleClassChecker`]
 * member-projection pass: every visited class's
 * function / property / constructor / enum-entry declarations come
 * back as [MemberPayload]s with the right kind, return type,
 * nullability, visibility, override / abstract flags, and parameters.
 */
class AnalysisSessionMembersTest {

    @TempDir
    lateinit var tmp: Path

    @Test
    fun functionsCaptureNameKindReturnTypeAndParams() {
        writeKt(
            "Funcs.kt",
            """
            package com.acme.funcs

            class Greeter {
                fun greet(name: String): String = "hi, " + name
                fun nullableGreet(name: String?): String? = name
            }
            """.trimIndent(),
        )

        val classes = analyzeAndIndex()
        val greeter = classes["com.acme.funcs.Greeter"]
        assertNotNull(greeter)
        val greet = greeter.functionsByName()["greet"]
        assertNotNull(greet)
        assertEquals("function", greet.kind)
        assertEquals("kotlin.String", greet.returnType)
        assertEquals(false, greet.nullable)
        assertEquals(
            listOf(ParamPayload(name = "name", type = "kotlin.String", nullable = false)),
            greet.params,
        )

        val nullableGreet = greeter.functionsByName()["nullableGreet"]
        assertNotNull(nullableGreet)
        assertEquals("kotlin.String?", nullableGreet.returnType)
        assertEquals(true, nullableGreet.nullable)
        assertTrue(nullableGreet.params.single().nullable)
    }

    @Test
    fun propertiesAndConstructorsHaveCorrectKindAndParams() {
        writeKt(
            "Props.kt",
            """
            package com.acme.props

            class Point(val x: Int, val y: Int) {
                val sum: Int get() = x + y
                var label: String? = null
            }
            """.trimIndent(),
        )

        val classes = analyzeAndIndex()
        val point = classes["com.acme.props.Point"]
        assertNotNull(point)

        // Primary constructor surfaces with the canonical "<init>" name
        // and carries its value parameters.
        val ctor = point.members.singleOrNull { it.kind == "constructor" }
        assertNotNull(ctor, "constructor missing: ${point.members.map { it.name }}")
        assertEquals("<init>", ctor.name)
        assertEquals(
            listOf(
                ParamPayload(name = "x", type = "kotlin.Int", nullable = false),
                ParamPayload(name = "y", type = "kotlin.Int", nullable = false),
            ),
            ctor.params,
        )

        // Properties — including the synthetic backing properties for
        // val/var constructor parameters — appear with kind=property.
        val byName = point.propertiesByName()
        val sum = byName["sum"]
        assertNotNull(sum)
        assertEquals("property", sum.kind)
        assertEquals("kotlin.Int", sum.returnType)

        val label = byName["label"]
        assertNotNull(label)
        assertEquals("kotlin.String?", label.returnType)
        assertEquals(true, label.nullable)
    }

    @Test
    fun visibilityAndOverrideAndAbstractFlagsPropagate() {
        writeKt(
            "Flags.kt",
            """
            package com.acme.flags

            interface Greeter {
                fun greet(): String
            }
            abstract class Base : Greeter {
                abstract fun shout(): String
                protected open fun guarded() {}
                private fun secret() {}
                internal fun pkgwide() {}
            }
            class Concrete : Base() {
                override fun greet() = "hi"
                override fun shout() = "HI"
            }
            """.trimIndent(),
        )

        val classes = analyzeAndIndex()

        // Concrete.greet overrides Greeter.greet — isOverride must surface.
        val concrete = classes["com.acme.flags.Concrete"]
        assertNotNull(concrete)
        val greet = concrete.functionsByName()["greet"]
        assertNotNull(greet)
        assertEquals(true, greet.isOverride)
        assertEquals(false, greet.isAbstract)

        // Base.shout is abstract; visibility is public, isAbstract = true.
        val base = classes["com.acme.flags.Base"]
        assertNotNull(base)
        val shout = base.functionsByName()["shout"]
        assertNotNull(shout)
        assertEquals(true, shout.isAbstract)

        // Each access modifier renders to the krit-types wire string.
        assertEquals("protected", base.functionsByName()["guarded"]?.visibility)
        assertEquals("private", base.functionsByName()["secret"]?.visibility)
        assertEquals("internal", base.functionsByName()["pkgwide"]?.visibility)
    }

    @Test
    fun enumEntriesAppearAsMembersOfTheEnumClass() {
        writeKt(
            "Color.kt",
            """
            package com.acme.color

            enum class Color { RED, GREEN, BLUE }
            """.trimIndent(),
        )

        val classes = analyzeAndIndex()
        val color = classes["com.acme.color.Color"]
        assertNotNull(color)
        val entries = color.members.filter { it.kind == "enum_entry" }.map { it.name }
        assertEquals(listOf("RED", "GREEN", "BLUE"), entries)
    }

    @Test
    fun classWithNoMembersStillEmitsEmptyMembersList() {
        writeKt(
            "Marker.kt",
            """
            package com.acme.marker

            class Marker
            """.trimIndent(),
        )

        val classes = analyzeAndIndex()
        val marker = classes["com.acme.marker.Marker"]
        assertNotNull(marker)
        // The default no-arg constructor is synthesized by K2 — it shows
        // up in `declarations` as a `FirConstructor` and we surface it.
        // Anything else (synthetic equals / hashCode / toString on
        // non-data classes) is skipped by our `toMemberPayload` filter.
        val nonCtor = marker.members.filter { it.kind != "constructor" }
        assertEquals(emptyList(), nonCtor, "non-ctor members: $nonCtor")
    }

    private fun analyzeAndIndex(): Map<String, ClassPayload> {
        val result = AnalysisSession(
            sourceDirs = listOf(tmp.toFile().absolutePath),
            classpath = emptyList(),
        ).analyze(emptyList())
        return result.dependencies
    }

    private fun ClassPayload.functionsByName(): Map<String, MemberPayload> =
        members.filter { it.kind == "function" }.associateBy { it.name }

    private fun ClassPayload.propertiesByName(): Map<String, MemberPayload> =
        members.filter { it.kind == "property" }.associateBy { it.name }

    private fun writeKt(name: String, source: String): String {
        val file = tmp.resolve(name).toFile()
        file.writeText(source)
        return file.canonicalPath
    }
}
