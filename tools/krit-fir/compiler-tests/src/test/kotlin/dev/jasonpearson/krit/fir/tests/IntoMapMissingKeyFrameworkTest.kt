package dev.jasonpearson.krit.fir.tests

import dev.jasonpearson.krit.fir.FirRuleCompileContext
import org.junit.jupiter.api.Test
import kotlin.test.assertEquals
import kotlin.test.assertTrue

// IntoMapMissingKey cases a single-file golden cannot express: annotations
// from other DI frameworks. The shared stubs have Metro's Provides and Binds
// but not its IntoMap or map keys, so those are declared here.
class IntoMapMissingKeyFrameworkTest {

    private fun findings(sources: Map<String, String>): List<Pair<String, Int>> {
        val result = KritFirProbe.compile(
            sources,
            FirRuleCompileContext(enabledRuleIds = setOf("IntoMapMissingKey")),
        )
        assertTrue(result.clean, result.problems())
        return result.diags.filter { it.name == "IntoMapMissingKey" }.map { it.file to it.line }
    }

    private val metroMultibindings = "MetroMultibindings.kt" to """
        package dev.zacsweers.metro

        @Target(AnnotationTarget.ANNOTATION_CLASS)
        annotation class MapKey(val unwrapValue: Boolean = true)

        @Target(AnnotationTarget.FUNCTION, AnnotationTarget.PROPERTY, AnnotationTarget.PROPERTY_GETTER)
        annotation class IntoMap

        @Target(AnnotationTarget.FUNCTION, AnnotationTarget.PROPERTY, AnnotationTarget.PROPERTY_GETTER)
        @MapKey
        annotation class StringKey(val value: String)
    """.trimIndent()

    // Metro, like Dagger, requires a map key on every @IntoMap binding. Go
    // matches the annotation names, so it reports the keyless functions on
    // lines 9 and 13 (a true positive FIR keeps) and not the keyed ones.
    @Test fun metroMapContributions() {
        val sources = mapOf(
            metroMultibindings,
            "Graph.kt" to """
                package demo

                import dev.zacsweers.metro.Binds
                import dev.zacsweers.metro.IntoMap
                import dev.zacsweers.metro.Provides
                import dev.zacsweers.metro.StringKey

                interface Bindings {
                    @Provides
                    @IntoMap
                    fun provideKeyless(): CharSequence = "a"

                    @Binds
                    @IntoMap
                    fun String.bindKeyless(): CharSequence

                    @Provides
                    @IntoMap
                    @StringKey("b")
                    fun provideKeyed(): CharSequence = "b"
                }
            """.trimIndent(),
        )
        assertEquals(listOf("Graph.kt" to 9, "Graph.kt" to 13), findings(sources))
    }

    // Divergence (precision): kotlin-inject's @IntoMap contributes a
    // Pair<K, V> and has no key annotation. Go matches the names @Provides and
    // @IntoMap and reports line 9; the message ("Dagger requires a key
    // annotation on every map contribution") is not true of this code.
    @Test fun kotlinInjectMapContribution() {
        val sources = mapOf(
            "KotlinInject.kt" to """
                package me.tatarka.inject.annotations

                annotation class Provides

                annotation class IntoMap
            """.trimIndent(),
            "Component.kt" to """
                package demo

                import me.tatarka.inject.annotations.IntoMap
                import me.tatarka.inject.annotations.Provides

                interface Handler

                interface HandlerComponent {
                    @Provides
                    @IntoMap
                    fun provideHandler(): Pair<String, Handler> = "a" to object : Handler {}
                }
            """.trimIndent(),
        )
        assertEquals(emptyList(), findings(sources))
    }
}
