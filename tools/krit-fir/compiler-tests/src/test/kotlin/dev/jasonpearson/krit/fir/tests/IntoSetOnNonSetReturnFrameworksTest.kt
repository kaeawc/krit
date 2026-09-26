package dev.jasonpearson.krit.fir.tests

import dev.jasonpearson.krit.fir.FirRuleCompileContext
import org.junit.jupiter.api.Test
import kotlin.test.assertEquals
import kotlin.test.assertTrue

// IntoSetOnNonSetReturn on the other DI frameworks whose @IntoSet collects a
// set by the provider's return type: Metro and kotlin-inject. Go matches the
// annotation text, so it reports both. The stubs have no Metro `IntoSet` and
// no kotlin-inject package, so this test declares those annotations (with the
// real packages, names, and targets) in a source of its own.
class IntoSetOnNonSetReturnFrameworksTest {

    private fun findings(sources: Map<String, String>): List<Pair<String, Int>> {
        val result = KritFirProbe.compile(
            sources,
            FirRuleCompileContext(enabledRuleIds = setOf("IntoSetOnNonSetReturn")),
        )
        assertTrue(result.clean, result.problems())
        return result.diags.filter { it.name == "IntoSetOnNonSetReturn" }.map { it.file to it.line }
    }

    private val metroIntoSet = "MetroIntoSet.kt" to """
        package dev.zacsweers.metro

        @Target(AnnotationTarget.FUNCTION, AnnotationTarget.PROPERTY, AnnotationTarget.PROPERTY_GETTER)
        annotation class IntoSet
    """.trimIndent()

    private val kotlinInject = "KotlinInject.kt" to """
        package me.tatarka.inject.annotations

        @Target(AnnotationTarget.FUNCTION, AnnotationTarget.PROPERTY_GETTER)
        annotation class Provides

        @Target(AnnotationTarget.FUNCTION, AnnotationTarget.PROPERTY_GETTER)
        annotation class IntoSet
    """.trimIndent()

    // Both are Go findings too (Go matches `@Provides` / `@Binds` and
    // `@IntoSet` by text); line 11's single element is left alone by both.
    @Test fun metro() {
        val sources = mapOf(
            metroIntoSet,
            "Graph.kt" to """
                package demo

                import dev.zacsweers.metro.Binds
                import dev.zacsweers.metro.IntoSet
                import dev.zacsweers.metro.Provides

                interface Plugin
                class PluginImpl : Plugin

                interface PluginGraph {
                    @Provides @IntoSet fun single(): Plugin = PluginImpl()

                    @Provides
                    @IntoSet
                    fun plugins(): List<Plugin> = emptyList()

                    @Binds
                    @IntoSet
                    fun ArrayList<Plugin>.bind(): Set<Plugin>
                }
            """.trimIndent(),
        )
        assertEquals(listOf("Graph.kt" to 13, "Graph.kt" to 17), findings(sources))
    }

    @Test fun kotlinInject() {
        val sources = mapOf(
            kotlinInject,
            "Component.kt" to """
                package demo

                import me.tatarka.inject.annotations.IntoSet
                import me.tatarka.inject.annotations.Provides

                interface Plugin
                class PluginImpl : Plugin

                abstract class PluginComponent {
                    @Provides @IntoSet fun single(): Plugin = PluginImpl()

                    @Provides
                    @IntoSet
                    fun plugins(): Map<String, Plugin> = emptyMap()
                }
            """.trimIndent(),
        )
        assertEquals(listOf("Component.kt" to 12), findings(sources))
    }
}
