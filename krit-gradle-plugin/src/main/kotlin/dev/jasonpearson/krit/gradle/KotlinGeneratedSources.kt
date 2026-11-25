package dev.jasonpearson.krit.gradle

import org.gradle.api.Project
import org.gradle.api.file.FileCollection
import org.gradle.api.file.SourceDirectorySet
import org.gradle.api.provider.Provider
import org.gradle.api.tasks.SourceSetContainer

/**
 * Source roots and their producer tasks for JVM and Kotlin Multiplatform compilations, from the
 * Kotlin Gradle plugin's source sets: `kotlin` and, on KGP 2.3+, `generatedKotlin`. Code
 * generators (KSP, SQLDelight, Wire) register their output there with `srcDir(taskProvider)`, so
 * the directory sets' `sourceDirectories` carry the producing tasks as build dependencies, the
 * same way AGP's Variant API does for Android (see [AndroidVariantSources]).
 *
 * The plugin has no compile dependency on KGP; source sets are reached by reflection, and a
 * source set without these properties contributes nothing.
 */
internal object KotlinGeneratedSources {
    /** The `kotlin` and `generatedKotlin` directory sets of a KGP source set. */
    fun directorySets(kotlinSourceSet: Any): List<SourceDirectorySet> =
        listOf("getKotlin", "getGeneratedKotlin").mapNotNull { getter ->
            runCatching { kotlinSourceSet.javaClass.getMethod(getter).invoke(kotlinSourceSet) as? SourceDirectorySet }.getOrNull()
        }

    /** The KGP source set named [name] of a JVM project, or null without the Kotlin plugin. */
    fun jvmSourceSet(project: Project, name: String): Any? {
        val kotlin = project.extensions.findByName("kotlin") ?: return null
        val sourceSets = runCatching { kotlin.javaClass.getMethod("getSourceSets").invoke(kotlin) }.getOrNull() ?: return null
        return runCatching {
            sourceSets.javaClass.getMethod("findByName", String::class.java).invoke(sourceSets, name)
        }.getOrNull()
    }

    /**
     * The roots of [sets] as model paths, read when the export runs, after every plugin had the
     * chance to register a generated directory. Reading them runs no task.
     */
    fun paths(project: Project, sets: List<SourceDirectorySet>): Provider<List<String>> =
        project.provider { sets.flatMap { set -> set.srcDirs.map(::modelPath) }.distinct().sorted() }

    /** The roots of [sets] (and the Java source set [javaSourceSet], if any) with their producer tasks. */
    fun producers(project: Project, sets: List<SourceDirectorySet>, javaSourceSet: String? = null): FileCollection {
        val java = javaSourceSet?.let { name ->
            project.extensions.findByType(SourceSetContainer::class.java)?.findByName(name)?.java?.sourceDirectories
        }
        return project.files(sets.map { it.sourceDirectories } + listOfNotNull(java))
    }
}
