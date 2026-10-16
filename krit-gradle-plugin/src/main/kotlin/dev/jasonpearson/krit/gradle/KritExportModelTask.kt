package dev.jasonpearson.krit.gradle

import groovy.json.JsonOutput
import org.gradle.api.DefaultTask
import org.gradle.work.DisableCachingByDefault
import org.gradle.api.file.ConfigurableFileCollection
import org.gradle.api.file.RegularFileProperty
import org.gradle.api.provider.ListProperty
import org.gradle.api.provider.Property
import org.gradle.api.tasks.Classpath
import org.gradle.api.tasks.Input
import org.gradle.api.tasks.Nested
import org.gradle.api.tasks.OutputFile
import org.gradle.api.tasks.TaskAction
import java.io.File

internal fun modelPath(file: File): String = file.absoluteFile.normalize().path.replace('\\', '/')

internal fun pickAndroidVariant(available: List<String>, requested: String): String? =
    available.firstOrNull { it == requested } ?: available.sorted().firstOrNull()

internal fun bootClasspathPaths(files: List<File>): List<String> =
    files.map(::modelPath).distinct().sorted()

private fun writeJson(file: File, value: Map<String, Any>) {
    file.parentFile.mkdirs()
    file.writeText(JsonOutput.prettyPrint(JsonOutput.toJson(value)) + "\n")
}

abstract class KritModelSourceSet {
    @get:Input abstract val name: Property<String>
    @get:Input abstract val kind: Property<String>
    @get:Input abstract val platform: Property<String>
    @get:Input abstract val variant: Property<String>
    @get:Input abstract val sourceDirs: ListProperty<String>
    @get:Classpath abstract val classpath: ConfigurableFileCollection
    @get:Input abstract val generatedClasspath: ListProperty<String>
    @get:Input abstract val bootClasspath: ListProperty<String>
    @get:Input abstract val projectDeps: ListProperty<String>
    @get:Input abstract val jvmTarget: Property<String>
    // Kotlin compiler settings as configured on the compilation. Blank / empty
    // means "not configured"; krit resolves the effective defaults (see
    // docs/gradle-model.md), so these are never filled in here.
    // False for a source set with no Kotlin compile task (Java-only): it then has no
    // `kotlin` object, so krit does not read "no settings" as a setting.
    @get:Input abstract val kotlinPresent: Property<Boolean>
    @get:Input abstract val kotlinLanguageVersion: Property<String>
    @get:Input abstract val kotlinApiVersion: Property<String>
    @get:Input abstract val kotlinOptIn: ListProperty<String>
    @get:Input abstract val kotlinFreeCompilerArgs: ListProperty<String>
    @get:Input abstract val kotlinProgressiveMode: Property<Boolean>
    @get:Input abstract val kotlinPluginVersion: Property<String>
}

/** Model schema written by [KritExportModelTask]. Schema 1 had no `kotlin` object. */
internal const val MODEL_SCHEMA = 2

@DisableCachingByDefault(because = "Output contains absolute artifact paths")
abstract class KritExportModelTask : DefaultTask() {
    @get:Nested abstract val sourceSets: ListProperty<KritModelSourceSet>
    @get:Input abstract val buildDir: Property<String>
    @get:Input abstract val projectPath: Property<String>
    @get:Input abstract val projectDir: Property<String>
    @get:Input abstract val generatedBy: Property<String>
    @get:Input abstract val rootDir: Property<String>
    @get:OutputFile abstract val outputFile: RegularFileProperty

    @TaskAction
    fun export() {
        val build = File(buildDir.get()).absoluteFile.normalize().toPath()
        val entries = sourceSets.get().map { sourceSet ->
            val (generated, ordinary) = sourceSet.sourceDirs.get().distinct().sorted().partition {
                File(it).absoluteFile.normalize().toPath().startsWith(build)
            }
            linkedMapOf<String, Any>(
                "name" to sourceSet.name.get(),
                "kind" to sourceSet.kind.get(),
                "platform" to sourceSet.platform.get(),
                "variant" to sourceSet.variant.get(),
                "sourceDirs" to ordinary,
                "generatedSourceDirs" to generated,
                "classpath" to sourceSet.classpath.files.map(::modelPath).distinct().sorted(),
                "generatedClasspath" to sourceSet.generatedClasspath.get().distinct().sorted(),
                "bootClasspath" to sourceSet.bootClasspath.get().distinct().sorted(),
                "projectDeps" to sourceSet.projectDeps.get().distinct().sorted(),
                "jvmTarget" to sourceSet.jvmTarget.get(),
            ).also { entry ->
                if (sourceSet.kotlinPresent.getOrElse(false)) {
                    entry["kotlin"] = linkedMapOf<String, Any>(
                        "languageVersion" to sourceSet.kotlinLanguageVersion.getOrElse(""),
                        "apiVersion" to sourceSet.kotlinApiVersion.getOrElse(""),
                        "optIn" to sourceSet.kotlinOptIn.getOrElse(emptyList()).distinct().sorted(),
                        // Order is significant (later flags win), so it is preserved.
                        "freeCompilerArgs" to sourceSet.kotlinFreeCompilerArgs.getOrElse(emptyList()),
                        "progressiveMode" to sourceSet.kotlinProgressiveMode.getOrElse(false),
                        "pluginVersion" to sourceSet.kotlinPluginVersion.getOrElse(""),
                    )
                }
            }
        }
        val entry = linkedMapOf<String, Any>(
            "path" to projectPath.get(),
            "dir" to projectDir.get(),
            "sourceSets" to entries,
        )
        writeJson(outputFile.get().asFile, linkedMapOf(
            "schema" to MODEL_SCHEMA,
            "generatedBy" to generatedBy.get(),
            "rootDir" to rootDir.get(),
            "projects" to listOf(entry),
        ))
    }
}
