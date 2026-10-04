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

    // Third-party Java collections whose simple names are in Go's wrapper
    // list: Eclipse Collections' MutableList (a java.util.List) and Vavr's
    // HashMap and List (java.lang.Iterable through Traversable). Go reports
    // all three by name, and each is a collection, so FIR keeps them. A
    // golden cannot declare Java, so the Java shapes live here, in their real
    // packages.
    @Test fun thirdPartyJavaCollections() {
        val sources = mapOf(
            "org/eclipse/collections/api/list/MutableList.java" to """
                package org.eclipse.collections.api.list;

                public interface MutableList<T> extends java.util.List<T> {
                }
            """.trimIndent(),
            "io/vavr/Tuple2.java" to """
                package io.vavr;

                public final class Tuple2<T1, T2> {
                }
            """.trimIndent(),
            "io/vavr/collection/Traversable.java" to """
                package io.vavr.collection;

                public interface Traversable<T> extends Iterable<T> {
                }
            """.trimIndent(),
            "io/vavr/collection/Map.java" to """
                package io.vavr.collection;

                import io.vavr.Tuple2;

                public interface Map<K, V> extends Traversable<Tuple2<K, V>> {
                }
            """.trimIndent(),
            "io/vavr/collection/HashMap.java" to """
                package io.vavr.collection;

                import io.vavr.Tuple2;
                import java.util.Iterator;

                public final class HashMap<K, V> implements Map<K, V> {
                    @Override
                    public Iterator<Tuple2<K, V>> iterator() {
                        throw new RuntimeException("Stub!");
                    }
                }
            """.trimIndent(),
            "io/vavr/collection/List.java" to """
                package io.vavr.collection;

                public interface List<T> extends Traversable<T> {
                }
            """.trimIndent(),
            "Module.kt" to """
                package demo

                import dagger.Provides
                import dagger.multibindings.IntoSet
                import io.vavr.collection.HashMap
                import org.eclipse.collections.api.list.MutableList

                interface Plugin

                class PluginModule {
                    @Provides
                    @IntoSet
                    fun eclipse(list: MutableList<Plugin>): MutableList<Plugin> = list

                    @Provides
                    @IntoSet
                    fun vavr(map: HashMap<String, Plugin>): HashMap<String, Plugin> = map

                    @Provides
                    @IntoSet
                    fun vavrList(list: io.vavr.collection.List<Plugin>): io.vavr.collection.List<Plugin> = list

                    // Vavr's Tuple2 is not a collection, and not in the list.
                    @Provides
                    @IntoSet
                    fun tuple(pair: io.vavr.Tuple2<String, Plugin>): io.vavr.Tuple2<String, Plugin> = pair
                }
            """.trimIndent(),
        )
        assertEquals(
            listOf("Module.kt" to 11, "Module.kt" to 15, "Module.kt" to 19),
            findings(sources),
        )
    }
}
