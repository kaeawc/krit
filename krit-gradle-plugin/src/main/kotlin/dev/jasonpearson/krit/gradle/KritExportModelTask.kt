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

@DisableCachingByDefault(because = "Output contains absolute artifact paths")
abstract class KritExportModelTask : DefaultTask() {
    @get:Classpath abstract val classpath: ConfigurableFileCollection
    @get:Input abstract val bootClasspath: ListProperty<String>
    @get:Input abstract val projectDeps: ListProperty<String>
    @get:Input abstract val sourceDirs: ListProperty<String>
    @get:Input abstract val projectPath: Property<String>
    @get:Input abstract val projectDir: Property<String>
    @get:Input abstract val platform: Property<String>
    @get:Input abstract val variant: Property<String>
    @get:Input abstract val sourceSetName: Property<String>
    @get:Input abstract val generatedBy: Property<String>
    @get:Input abstract val rootDir: Property<String>
    @get:Input abstract val hasModel: Property<Boolean>
    @get:OutputFile abstract val outputFile: RegularFileProperty

    @TaskAction
    fun export() {
        val sourceSet = linkedMapOf<String, Any>(
            "name" to sourceSetName.get(),
            "platform" to platform.get(),
            "variant" to variant.get(),
            "sourceDirs" to sourceDirs.get().distinct().sorted(),
            "classpath" to classpath.files.map(::modelPath).distinct().sorted(),
            "bootClasspath" to bootClasspath.get().distinct().sorted(),
            "projectDeps" to projectDeps.get().distinct().sorted(),
        )
        val entry = linkedMapOf<String, Any>(
            "path" to projectPath.get(),
            "dir" to projectDir.get(),
            "sourceSets" to if (hasModel.get()) listOf(sourceSet) else emptyList<Any>(),
        )
        writeJson(outputFile.get().asFile, linkedMapOf(
            "schema" to 1,
            "generatedBy" to generatedBy.get(),
            "rootDir" to rootDir.get(),
            "projects" to listOf(entry),
        ))
    }
}
