package dev.jasonpearson.krit.fir.support

import dev.jasonpearson.krit.fir.runner.AnalysisSession
import dev.jasonpearson.krit.fir.runner.FileRef
import dev.jasonpearson.krit.fir.runner.ModuleSpec
import dev.jasonpearson.krit.fir.checkers.licensing.isInOptInMarkerPackageFamily
import java.io.File
import java.nio.file.Path
import kotlin.test.Test
import kotlin.test.assertEquals
import org.junit.jupiter.api.io.TempDir
import org.jetbrains.kotlin.cli.common.ExitCode
import org.jetbrains.kotlin.cli.jvm.K2JVMCompiler

class FirDeclarationExposureTest {
    @TempDir lateinit var tmp: Path

    private fun findings(
        source: String,
        testFile: Boolean = false,
        ruleConfigs: Map<String, Map<String, Any?>> = emptyMap(),
        classpath: List<String> = emptyList(),
    ): List<Int> {
        val file = tmp.resolve("Sample.kt").toFile().apply { writeText(source) }
        val stdlib = File(kotlin.Unit::class.java.protectionDomain.codeSource.location.toURI()).absolutePath
        val result = AnalysisSession(listOf(tmp.toString()), listOf(stdlib) + classpath)
            .check(
                1, listOf(FileRef(file.absolutePath)), setOf("OptInMarkerExposedPublicly"), ruleConfigs,
                testFiles = if (testFile) setOf(file.absolutePath) else emptySet(),
                incremental = false,
            )
        assertEquals(1, result.succeeded, result.toString())
        return result.findings.filter { it.rule == "OptInMarkerExposedPublicly" }.map { it.line }.sorted()
    }

    @Test
    fun skipsUnconfiguredProjectMarkers() {
        assertEquals(emptyList(), findings("""
            @kotlin.RequiresOptIn(level = kotlin.RequiresOptIn.Level.WARNING)
            annotation class LocalMarker
            @LocalMarker
            fun samePackageMarker() = Unit
        """.trimIndent()))
    }

    @Test
    fun skipsKnownSourceMarkersInNestedLibraryPackage() {
        val marker = tmp.resolve("Marker.kt").toFile().apply {
            writeText("""
                package kotlinx.coroutines
                @kotlin.RequiresOptIn
                annotation class ExperimentalCoroutinesApi
            """.trimIndent())
        }
        val api = tmp.resolve("Api.kt").toFile().apply {
            writeText("""
                package kotlinx.coroutines.flow
                import kotlinx.coroutines.ExperimentalCoroutinesApi
                @ExperimentalCoroutinesApi
                fun exposed() = Unit
            """.trimIndent())
        }
        val stdlib = File(kotlin.Unit::class.java.protectionDomain.codeSource.location.toURI()).absolutePath
        val result = AnalysisSession(listOf(tmp.toString()), listOf(stdlib))
            .check(
                1,
                listOf(FileRef(marker.absolutePath), FileRef(api.absolutePath)),
                setOf("OptInMarkerExposedPublicly"),
                incremental = false,
            )
        assertEquals(emptyList(), result.findings.filter { it.rule == "OptInMarkerExposedPublicly" })
    }

    @Test
    fun reportsSourceDefinedKnownMarkersOutsideTheirLibraryPackageRootInBothCompileModes() {
        val stdlib = File(kotlin.Unit::class.java.protectionDomain.codeSource.location.toURI()).absolutePath
        val marker = tmp.resolve("Marker.kt").toFile().apply {
            writeText("""
                package com.acme.annotations
                @kotlin.RequiresOptIn
                annotation class ExperimentalCoroutinesApi
            """.trimIndent())
        }
        val api = tmp.resolve("Api.kt").toFile().apply {
            writeText("""
                package org.example.api
                import com.acme.annotations.ExperimentalCoroutinesApi
                @ExperimentalCoroutinesApi
                fun exposed() = Unit
            """.trimIndent())
        }
        val session = AnalysisSession(listOf(tmp.toString()), listOf(stdlib))
        try {
            val result = session.check(
                1,
                listOf(FileRef(marker.absolutePath), FileRef(api.absolutePath)),
                setOf("OptInMarkerExposedPublicly"),
                incremental = false,
            )
            assertEquals(2, result.succeeded, result.toString())
            assertEquals(listOf(3), result.findings.filter { it.rule == "OptInMarkerExposedPublicly" }.map { it.line })
            val modules = session.analyzeModules(
                2,
                listOf(ModuleSpec("project", sourceRoots = listOf(tmp.toString()), classpath = listOf(stdlib))),
                listOf(marker.absolutePath, api.absolutePath),
                setOf("OptInMarkerExposedPublicly"),
            )
            assertEquals(2, modules.succeeded, modules.toString())
            assertEquals(listOf(3), modules.findings.filter { it.rule == "OptInMarkerExposedPublicly" }.map { it.line })
        } finally {
            session.dispose()
        }
    }

    @Test
    fun skipsKnownSourceMarkersWithinTheirLibraryPackageRootInBothCompileModes() {
        val stdlib = File(kotlin.Unit::class.java.protectionDomain.codeSource.location.toURI()).absolutePath
        val markerName = "ExperimentalCoroutinesApi"
        val marker = tmp.resolve("Marker.kt").toFile().apply {
            writeText("""
                package com.acme.annotations
                @kotlin.RequiresOptIn
                annotation class $markerName
            """.trimIndent())
        }
        val api = tmp.resolve("Api.kt").toFile().apply {
            writeText("""
                package com.acme.network
                import com.acme.annotations.$markerName
                @$markerName
                fun exposed() = Unit
            """.trimIndent())
        }
        val session = AnalysisSession(listOf(tmp.toString()), listOf(stdlib))
        try {
            val result = session.check(
                1,
                listOf(FileRef(marker.absolutePath), FileRef(api.absolutePath)),
                setOf("OptInMarkerExposedPublicly"),
                incremental = false,
            )
            assertEquals(2, result.succeeded, result.toString())
            assertEquals(emptyList(), result.findings.filter { it.rule == "OptInMarkerExposedPublicly" })
            val modules = session.analyzeModules(
                2,
                listOf(ModuleSpec("project", sourceRoots = listOf(tmp.toString()), classpath = listOf(stdlib))),
                listOf(marker.absolutePath, api.absolutePath),
                setOf("OptInMarkerExposedPublicly"),
            )
            assertEquals(2, modules.succeeded, modules.toString())
            assertEquals(emptyList(), modules.findings.filter { it.rule == "OptInMarkerExposedPublicly" })
        } finally {
            session.dispose()
        }
    }

    @Test
    fun checksBinaryMarkersInTheirOwnPackageTree() {
        val coroutines = File(kotlinx.coroutines.Job::class.java.protectionDomain.codeSource.location.toURI()).absolutePath
        for (packageName in listOf("kotlinx.coroutines", "kotlinx.coroutines.extras", "example.api")) {
            val expected = if (packageName == "example.api") listOf(2) else emptyList()
            assertEquals(expected, findings("""
                package $packageName
                @kotlinx.coroutines.ExperimentalCoroutinesApi
                fun exposed() = Unit
            """.trimIndent(), classpath = listOf(coroutines)))
        }
    }

    @Test
    fun checksAdditionalMarkersBySimpleNameWithResolvedMetadata() {
        val librarySource = tmp.resolve("library/Markers.kt").toFile().apply {
            parentFile.mkdirs()
            writeText("""
                package vendor
                @kotlin.RequiresOptIn
                annotation class ProjectApi
                annotation class ExperimentalTime
            """.trimIndent())
        }
        val stdlib = File(kotlin.Unit::class.java.protectionDomain.codeSource.location.toURI()).absolutePath
        val library = tmp.resolve("markers.jar").toString()
        assertEquals(ExitCode.OK, K2JVMCompiler().exec(
            System.err, "-no-stdlib", "-no-reflect", "-classpath", stdlib,
            "-d", library, librarySource.absolutePath,
        ))
        librarySource.delete()
        val source = """
            package sample.app
            import vendor.ProjectApi
            @ProjectApi
            fun exposed() = Unit
            @vendor.ExperimentalTime
            fun ordinaryAnnotation() = Unit
        """.trimIndent()
        assertEquals(emptyList(), findings(source, classpath = listOf(library)))
        for (configured in listOf("ProjectApi", "vendor.ProjectApi", "other.ProjectApi")) {
            assertEquals(listOf(3), findings(
                source,
                ruleConfigs = mapOf("OptInMarkerExposedPublicly" to mapOf("additionalMarkers" to listOf(configured))),
                classpath = listOf(library),
            ))
        }
        val configuredSourceMarker = """
            package test
            @RequiresOptIn annotation class InternalMyApi
            @InternalMyApi fun exposed() {}
        """.trimIndent()
        assertEquals(listOf(3), findings(
            configuredSourceMarker,
            ruleConfigs = mapOf("OptInMarkerExposedPublicly" to mapOf("additionalMarkers" to listOf("InternalMyApi"))),
        ))
        assertEquals(listOf(3), findings(
            source.replace("import vendor.ProjectApi", "import vendor.ProjectApi as Alias").replace("@ProjectApi", "@Alias"),
            ruleConfigs = mapOf("OptInMarkerExposedPublicly" to mapOf("additionalMarkers" to listOf("ProjectApi"))),
            classpath = listOf(library),
        ))
        assertEquals(emptyList(), findings(
            source,
            ruleConfigs = mapOf("OptInMarkerExposedPublicly" to mapOf("additionalMarkers" to listOf("UnrelatedApi"))),
            classpath = listOf(library),
        ))
        assertEquals(listOf(3), findings(
            source.replace("package sample.app", "package vendor.extras"),
            ruleConfigs = mapOf("OptInMarkerExposedPublicly" to mapOf("additionalMarkers" to listOf("ProjectApi"))),
            classpath = listOf(library),
        ))
        assertEquals(emptyList(), findings(
            source,
            ruleConfigs = mapOf("OptInMarkerExposedPublicly" to mapOf("additionalMarkers" to listOf("ExperimentalTime"))),
            classpath = listOf(library),
        ))
        val bobLibrarySource = tmp.resolve("bob/Markers.kt").toFile().apply {
            parentFile.mkdirs()
            writeText("""
                package io.github.bob.lib
                @kotlin.RequiresOptIn
                annotation class ExperimentalBobApi
            """.trimIndent())
        }
        val bobLibrary = tmp.resolve("bob-markers.jar").toString()
        assertEquals(ExitCode.OK, K2JVMCompiler().exec(
            System.err, "-no-stdlib", "-no-reflect", "-classpath", stdlib,
            "-d", bobLibrary, bobLibrarySource.absolutePath,
        ))
        bobLibrarySource.delete()
        assertEquals(listOf(3), findings("""
            package io.github.alice.app
            import io.github.bob.lib.ExperimentalBobApi
            @ExperimentalBobApi fun exposed() = Unit
        """.trimIndent(), ruleConfigs = mapOf(
            "OptInMarkerExposedPublicly" to mapOf("additionalMarkers" to listOf("ExperimentalBobApi")),
        ), classpath = listOf(bobLibrary)))
    }

    @Test
    fun packageFamiliesSkipSiblingApisButNotSharedOrganizationApps() {
        assertEquals(true, isInOptInMarkerPackageFamily("kotlin.native", "kotlin.experimental"))
        assertEquals(false, isInOptInMarkerPackageFamily("coil3.request", "coil.annotation"))
        assertEquals(false, isInOptInMarkerPackageFamily("com.google.accompanistx", "com.google.accompanist.permissions"))
        for ((markerPackage, apiPackage, markerName, expected) in listOf(
            listOf("coil.annotation", "coil.compose", "ExperimentalCoilApi", "silent"),
            listOf("coil3.annotation", "coil3.request", "ExperimentalCoilApi", "silent"),
            listOf("com.google.accompanist.permissions", "com.google.samples.apps.myapp", "ExperimentalPermissionsApi", "report"),
            listOf("com.google.android.horologist.annotations", "com.google.samples.apps.myapp", "ExperimentalHorologistApi", "report"),
        )) {
            val marker = tmp.resolve("Marker.kt").toFile().apply {
                writeText("package $markerPackage\n@kotlin.RequiresOptIn\nannotation class $markerName")
            }
            val source = """
                package $apiPackage
                @$markerPackage.$markerName
                fun exposed() = Unit
            """.trimIndent()
            val expectedLines = if (expected == "report") listOf(2) else emptyList()
            assertEquals(expectedLines, findings(source))
            val stdlib = File(kotlin.Unit::class.java.protectionDomain.codeSource.location.toURI()).absolutePath
            val library = tmp.resolve("marker.jar").toString()
            assertEquals(ExitCode.OK, K2JVMCompiler().exec(
                System.err, "-no-stdlib", "-no-reflect", "-classpath", stdlib,
                "-d", library, marker.absolutePath,
            ))
            marker.delete()
            assertEquals(expectedLines, findings(source, classpath = listOf(library)))
        }
    }

    @Test
    fun requiresOptInNameIsPlumbingUnlessExplicitlyConfigured() {
        tmp.resolve("Marker.kt").toFile().writeText("""
            package vendor.annotations
            @kotlin.RequiresOptIn
            annotation class RequiresOptIn
        """.trimIndent())
        val source = """
            package sample.api
            @vendor.annotations.RequiresOptIn
            fun exposed() = Unit
        """.trimIndent()
        assertEquals(emptyList(), findings(source))
        assertEquals(listOf(2), findings(
            source,
            ruleConfigs = mapOf("OptInMarkerExposedPublicly" to mapOf("additionalMarkers" to listOf("RequiresOptIn"))),
        ))
    }

    @Test
    fun goSilentCandidatesAreLeftToTheConfirmOnlyVerdict() {
        assertEquals(listOf(7, 11, 14, 18), findings("""
            package sample
            import kotlin.ExperimentalStdlibApi
            import kotlin.ExperimentalStdlibApi as Alias

            annotation class OptIn

            @Alias
            fun aliased() = Unit

            @Suppress("unused")
            @ExperimentalStdlibApi
            val stacked = 1
            @OptIn
            @ExperimentalStdlibApi
            fun lookalikeAcknowledgment() = Unit

            class Owner {
                @ExperimentalStdlibApi
                companion object
            }
        """.trimIndent()))
    }

    @Test
    fun effectiveVisibilityExcludesProtectedDeclarationsAndHiddenOwnersOrLocals() {
        assertEquals(emptyList(), findings("""
            open class PublicOwner {
                @kotlin.ExperimentalStdlibApi
                protected fun exposed() = Unit
                companion object {
                    @kotlin.ExperimentalStdlibApi
                    protected fun companionHelper() = Unit
                }
            }
            internal class HiddenOwner {
                @kotlin.ExperimentalStdlibApi
                fun hidden() = Unit
            }
            fun localScope() {
                @kotlin.ExperimentalStdlibApi
                fun local() = Unit
            }
        """.trimIndent()))
    }

    @Test
    fun protectedDeclarationsAndSealedConstructorsAreNotExternallyReachable() {
        assertEquals(emptyList(), findings("""
            class FinalOwner {
                @kotlin.ExperimentalStdlibApi
                protected fun helper() = Unit
                protected class Nested {
                    @kotlin.ExperimentalStdlibApi
                    fun helper() = Unit
                }
            }
            enum class Mode {
                A;
                @kotlin.ExperimentalStdlibApi
                protected fun helper() = Unit
            }
            sealed class Shape {
                @kotlin.ExperimentalStdlibApi
                protected fun helper() = Unit
                @kotlin.ExperimentalStdlibApi
                constructor(id: Int)
            }
        """.trimIndent()))
    }

    @Test
    fun testFileClassificationSkipsOtherwiseExposedMarkers() {
        val source = """
            @kotlin.ExperimentalStdlibApi
            fun exposed() = Unit
        """.trimIndent()
        assertEquals(listOf(1), findings(source))
        assertEquals(emptyList(), findings(source, testFile = true))
    }

    @Test
    fun acknowledgesResolvedOptInAndAliasedSupertypeAnnotations() {
        assertEquals(emptyList(), findings("""
            package sample
            import kotlin.OptIn as Acknowledge
            import kotlin.ExperimentalStdlibApi
            import sample.Base as RenamedBase
            @Acknowledge(ExperimentalStdlibApi::class)
            @ExperimentalStdlibApi
            interface Base
            @ExperimentalStdlibApi
            class Child : RenamedBase
        """.trimIndent()))
    }
}
