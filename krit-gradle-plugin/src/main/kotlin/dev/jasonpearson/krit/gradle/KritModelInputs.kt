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
    private const val GRADLE_USER_HOME = "\$GRADLE_USER_HOME"

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

    /**
     * Every classpath entry the models list, in model order. A predicted Android R jar
     * (`intermediates/<folder>/<component>/<task>/R.jar`, see KritPlugin.androidRJar) also
     * brings the component's directory under each folder AGP writes that jar to: krit uses
     * whichever R jar is there when it loads the model (gradlemodel.resolveRClassJars), so a
     * jar written next to the prediction must change the key too. Missing directories
     * fingerprint as empty.
     */
    fun classpath(models: Collection<File>): List<File> =
        models.filter { it.isFile }.sortedBy { it.name }.flatMap { file ->
            buildList { collectClasspath(JsonSlurper().parse(file), classpath = false, into = this) }
        }.flatMap { listOf(it) + rClassJarDirectories(it) }.distinct().map(::File)

    /** AGP's folders for a module's own R jar, grouped by artifact (as in gradlemodel/rjar.go). */
    private val R_CLASS_JAR_FOLDERS = listOf(
        listOf("compile_and_runtime_not_namespaced_r_class_jar", "compile_and_runtime_r_class_jar"),
        listOf("compile_r_class_jar"),
    )

    private fun rClassJarDirectories(entry: String): List<String> {
        val parts = entry.split('/')
        if (parts.size < 5 || parts.last() != "R.jar" || parts[parts.size - 5] != "intermediates") return emptyList()
        val group = R_CLASS_JAR_FOLDERS.firstOrNull { parts[parts.size - 4] in it } ?: return emptyList()
        val intermediates = parts.subList(0, parts.size - 4).joinToString("/")
        val component = parts[parts.size - 3]
        return group.map { "$intermediates/$it/$component" }
    }

    private fun normalize(value: Any?, root: String?, classpath: Boolean): Any? = when (value) {
        is Map<*, *> -> value.entries.associate { (key, entry) ->
            // The user home only locates the jars; their identity is kept by $GRADLE_CACHES.
            key.toString() to
                if (key == "gradleUserHome") GRADLE_USER_HOME else normalize(entry, root, key in CLASSPATH_KEYS)
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
