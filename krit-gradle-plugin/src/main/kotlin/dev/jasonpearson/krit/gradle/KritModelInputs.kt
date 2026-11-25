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
 * [normalized], the JSON with paths made location-independent (under the model's
 * `rootDir`, `$ROOT/…`; under a Gradle user home's dependency caches, whose
 * layout is content-addressed, `$GRADLE_CACHES/…`) so each source set's classpath
 * keeps its identity, and [classpath], the jars themselves, hashed by content as a
 * `@Classpath` input.
 */
internal object KritModelInputs {
    private val CLASSPATH_KEYS = setOf("classpath", "bootClasspath", "generatedClasspath", "compilerPluginClasspath")
    private const val ROOT = "\$ROOT"
    private const val GRADLE_CACHES = "\$GRADLE_CACHES"

    /** `<gradle user home>/caches/` followed by one of Gradle's dependency or transform caches. */
    private val GRADLE_CACHE_PREFIX =
        Regex("^.*?/caches/(?=(modules-\\d+|transforms-\\d+|jars-\\d+|[0-9][^/]*/transforms)/)")

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
        // Content is tracked by classpath(); here each entry keeps its identity (many
        // jars share a name, such as Android's classes.jar), just not its location.
        is String -> when {
            root != null && (value == root || value.startsWith("$root/")) -> ROOT + value.removePrefix(root)
            classpath -> GRADLE_CACHE_PREFIX.find(value)
                ?.let { GRADLE_CACHES + "/" + value.substring(it.range.last + 1) } ?: value
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
