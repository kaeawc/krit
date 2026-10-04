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
}

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
            )
        }
        val entry = linkedMapOf<String, Any>(
            "path" to projectPath.get(),
            "dir" to projectDir.get(),
            "sourceSets" to entries,
        )
        writeJson(outputFile.get().asFile, linkedMapOf(
            "schema" to 1,
            "generatedBy" to generatedBy.get(),
            "rootDir" to rootDir.get(),
            "projects" to listOf(entry),
        ))
    }
}
