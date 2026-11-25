package dev.jasonpearson.krit.gradle

import groovy.json.JsonOutput
import groovy.json.JsonSlurper
import java.io.File

/**
 * Build-cache view of exported Gradle model files.
 *
 * A model holds absolute paths: the build root, source directories, and classpath
 * jars under the Gradle user home. Hashing it as a file would give checkouts at
 * different paths different cache keys and would miss a jar whose content changed
 * at the same path (a re-published SNAPSHOT). So tasks key the model in two parts:
 * [normalized], the JSON with paths under the model's `rootDir` made root-relative
 * and classpath entries reduced to file names, and [classpath], the jars
 * themselves, hashed by content as a `@Classpath` input.
 */
internal object KritModelInputs {
    private val CLASSPATH_KEYS = setOf("classpath", "bootClasspath", "generatedClasspath", "compilerPluginClasspath")
    private const val ROOT = "\$ROOT"

    /** Root-relative model content, one entry per file, ordered by file name. */
    fun normalized(models: Collection<File>): String =
        models.filter { it.isFile }.sortedBy { it.name }.joinToString("\n") { file ->
            val model = JsonSlurper().parse(file)
            val root = ((model as? Map<*, *>)?.get("rootDir") as? String)?.trimEnd('/')
            file.name + " " + JsonOutput.toJson(normalize(model, root, classpath = false))
        }

    /** Every classpath entry the models list, in model order. */
    fun classpath(models: Collection<File>): List<File> =
        models.filter { it.isFile }.sortedBy { it.name }.flatMap { file ->
            buildList { collectClasspath(JsonSlurper().parse(file), classpath = false, into = this) }
        }.distinct().map(::File)

    private fun normalize(value: Any?, root: String?, classpath: Boolean): Any? = when (value) {
        is Map<*, *> -> value.entries.associate { (key, entry) ->
            key.toString() to normalize(entry, root, key in CLASSPATH_KEYS)
        }
        is List<*> -> value.map { normalize(it, root, classpath) }
        is String -> when {
            // Content is tracked by classpath(); only the entry's identity stays here.
            classpath -> File(value).name
            root != null && (value == root || value.startsWith("$root/")) -> ROOT + value.removePrefix(root)
            else -> value
        }
        else -> value
    }

    private fun collectClasspath(value: Any?, classpath: Boolean, into: MutableList<String>) {
        when (value) {
            is Map<*, *> -> value.forEach { (key, entry) -> collectClasspath(entry, key in CLASSPATH_KEYS, into) }
            is List<*> -> value.forEach { collectClasspath(it, classpath, into) }
            is String -> if (classpath) into += value
        }
    }
}
