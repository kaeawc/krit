package dev.jasonpearson.krit.fir.runner

import dev.jasonpearson.krit.fir.incremental.FingerprintBuilder
import org.jetbrains.kotlin.cli.common.ExitCode
import org.jetbrains.kotlin.cli.common.arguments.K2JVMCompilerArguments
import org.jetbrains.kotlin.cli.common.messages.CompilerMessageSeverity
import org.jetbrains.kotlin.cli.common.messages.CompilerMessageSourceLocation
import org.jetbrains.kotlin.cli.common.messages.MessageCollector
import org.jetbrains.kotlin.config.Services
import java.io.File
import java.nio.file.Files
import java.util.concurrent.ConcurrentHashMap

/**
 * The project's Kotlin compiler plugins (kotlinx.serialization, Compose, Parcelize, all-open, no-arg, ...): the
 * plugin jars and their `plugin:<id>:<key>=<value>` options, as krit's Gradle model exporter (or `krit.yml`) supplied
 * them. Plugins generate declarations the frontend resolves against (`Foo.serializer()`), so without them a module
 * that uses one compiles with errors, and an erroneous module has no incremental snapshot.
 *
 * This is a krit-owned field of its own, never a compiler argument: `-Xplugin` / `-P` stay outside the
 * [KotlinCompilerSettings] allowlist. krit-fir's own plugin jar is always registered first; these come after it.
 *
 * A jar that does not exist and an option that is not `plugin:<id>:<key>=<value>` are dropped and reported in
 * [warnings]. The plugins that remain are loaded once in a probe compile ([verified]); a set the embedded compiler
 * cannot load (a plugin built for an incompatible compiler) is left out with a warning, so the analysis keeps
 * today's no-plugin behavior instead of failing every file.
 */
data class CompilerPlugins(
    val classpath: List<String> = emptyList(),
    val options: List<String> = emptyList(),
    /** What was not applied and why; never part of [identity]. */
    val warnings: List<String> = emptyList(),
) {
    /** True when no plugin would be loaded. */
    val isEmpty: Boolean get() = classpath.isEmpty()

    /**
     * Canonical form of the plugin set by path and option, for session comparison. Jar contents are not part of it:
     * [addTo] puts each jar's file identity into a compile key.
     */
    val identity: String
        get() = if (isEmpty) "" else "plugins=" + classpath.joinToString("\u001f") + "|options=" + options.joinToString("\u001f")

    /** Adds the plugin set to a compile key: every jar's file identity (path, size, mtime, entry CRCs) and every option. */
    internal fun addTo(builder: FingerprintBuilder): FingerprintBuilder {
        if (isEmpty) return builder
        for (jar in classpath) builder.addFileIdentity("compilerPlugin", jar)
        return builder.add("compilerPluginOptions", options)
    }

    /** Registers the plugins after [selfJar] (krit-fir's own plugin) on [args]. */
    fun applyTo(args: K2JVMCompilerArguments, selfJar: String?) {
        val jars = listOfNotNull(selfJar) + classpath.filter { it != selfJar }
        args.pluginClasspaths = jars.toTypedArray()
        args.pluginOptions = (if (classpath.isEmpty()) emptyList() else options).toTypedArray()
    }

    /**
     * This set if the embedded compiler can load it, otherwise the empty set with a warning saying why. The probe is
     * one frontend-only compile of a one-line file, run once per plugin set (by jar identity and options) per process.
     */
    fun verified(classpath: List<String>): CompilerPlugins {
        if (isEmpty) return this
        val key = addTo(FingerprintBuilder()).build()
        val failure = probes.computeIfAbsent(key) {
            // Logged once per plugin set, so a one-shot run (which has no response warning) says it too.
            probe(this, classpath).also { if (it.isNotEmpty()) System.err.println("krit-fir: $it") }
        }
        return if (failure.isEmpty()) this else CompilerPlugins(warnings = warnings + failure)
    }

    companion object {
        val EMPTY = CompilerPlugins()

        private val OPTION = Regex("^plugin:[^:=\\s]+:[^=\\s:]+=.*$", RegexOption.DOT_MATCHES_ALL)

        // Plugin set key -> "" when it loads, else the warning. Shared by sessions, rebuilds and module compiles.
        private val probes = ConcurrentHashMap<String, String>()

        internal fun clearProbes() = probes.clear()

        /**
         * The plugin set of [classpath] jars and [options], validated: missing jars and malformed options are
         * dropped with a warning, duplicates are removed, and options without any jar are not applied.
         */
        fun of(classpath: List<String>, options: List<String>): CompilerPlugins {
            val warnings = mutableListOf<String>()
            val jars = LinkedHashSet<String>()
            for (raw in classpath) {
                val entry = raw.trim()
                if (entry.isEmpty()) continue
                val file = File(entry)
                if (!file.isFile && !file.isDirectory) {
                    warnings += "ignoring compiler plugin $entry (not found)"
                    continue
                }
                jars += file.absolutePath
            }
            val applied = mutableListOf<String>()
            for (raw in options) {
                val option = raw.trim()
                if (option.isEmpty()) continue
                when {
                    !OPTION.matches(option) -> warnings += "ignoring compiler plugin option $option (expected plugin:<id>:<key>=<value>)"
                    option !in applied -> applied += option
                }
            }
            if (jars.isEmpty() && applied.isNotEmpty()) {
                warnings += "ignoring ${applied.size} compiler plugin option(s): no compiler plugin jar was supplied"
                applied.clear()
            }
            return CompilerPlugins(jars.toList(), applied, warnings)
        }

        private fun probe(plugins: CompilerPlugins, classpath: List<String>): String {
            val dir = Files.createTempDirectory("krit-fir-plugin-probe-").toFile()
            try {
                val source = File(dir, "KritCompilerPluginProbe.kt").apply { writeText("package krit.probe\n\nclass KritCompilerPluginProbe\n") }
                val args = K2JVMCompilerArguments().apply {
                    freeArgs = listOf(source.path)
                    this.classpath = effectiveClasspath(classpath).joinToString(File.pathSeparator)
                    destination = File(dir, "out").path
                    noStdlib = true
                    noReflect = true
                    pluginClasspaths = plugins.classpath.toTypedArray()
                    pluginOptions = plugins.options.toTypedArray()
                }
                val errors = mutableListOf<String>()
                val messages = object : MessageCollector {
                    override fun clear() = errors.clear()
                    override fun hasErrors() = errors.isNotEmpty()
                    override fun report(severity: CompilerMessageSeverity, message: String, location: CompilerMessageSourceLocation?) {
                        if (severity.isError) errors += message.lineSequence().first()
                    }
                }
                val exit = try {
                    FrontendOnlyJvmPipeline().execute(args, Services.EMPTY, messages)
                } catch (t: Throwable) {
                    if (t is VirtualMachineError || t is InterruptedException) throw t
                    errors += "${t.javaClass.simpleName}: ${t.message}"
                    ExitCode.INTERNAL_ERROR
                }
                if (exit == ExitCode.OK && errors.isEmpty()) return ""
                val reason = errors.firstOrNull() ?: "compiler exited with $exit"
                return "compiler plugins not loaded, analyzing without them (${plugins.classpath.joinToString(", ") { File(it).name }}: $reason)"
            } finally {
                dir.deleteRecursively()
            }
        }
    }
}
