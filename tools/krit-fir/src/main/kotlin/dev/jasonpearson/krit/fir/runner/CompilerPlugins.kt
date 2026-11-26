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
 * [warnings]. The plugins that remain are loaded once in a probe compile ([verified]). When the embedded compiler
 * cannot load the whole set (a plugin built for an incompatible compiler, or an embeddable build that refers to
 * relocated IntelliJ classes, like the embeddable Compose plugin), each plugin it cannot load is left out with a
 * warning naming it and the others are kept, so one plugin never takes kotlinx.serialization or Metro with it (#1598).
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
     * This set if the embedded compiler can load it. Otherwise the largest set it can load built up one jar at a time:
     * each jar is kept when it loads with the jars kept so far, and the jars that did not are tried again until a pass
     * keeps none, so a plugin split across jars loads even when its registrar jar comes before the jars it links
     * against. Each jar left out gets a warning saying why; the result is the empty set when none loads. A probe is one
     * frontend-only compile of a one-line file, run once per plugin set (by jar identity and options) per process, so
     * a set that loads costs one probe and one that does not about one more per jar and pass.
     */
    fun verified(classpath: List<String>): CompilerPlugins {
        if (isEmpty || probeCached(this, classpath).isEmpty()) return this
        val kept = ArrayList<String>()
        val failures = LinkedHashMap<String, String>()
        var pending = this.classpath
        do {
            val before = kept.size
            val retry = ArrayList<String>()
            for (jar in pending) {
                val failure = probeCached(copy(classpath = kept + jar), classpath)
                if (failure.isEmpty()) {
                    kept += jar
                } else {
                    failures[jar] = failure
                    retry += jar
                }
            }
            pending = retry
        } while (pending.isNotEmpty() && kept.size > before)
        val dropped = pending.map { "compiler plugin ${File(it).name} not loaded, analyzing without it (${failures.getValue(it)})" }
        // Logged once per plugin set, so a one-shot run (which has no response warning) says it too.
        if (logged.add(addTo(FingerprintBuilder()).build())) dropped.forEach { System.err.println("krit-fir: $it") }
        // An option of a plugin left out has no command-line processor to read it, and the compiler ignores it.
        val loaded = if (kept.isEmpty()) CompilerPlugins() else CompilerPlugins(kept, options)
        return loaded.copy(warnings = warnings + dropped)
    }

    companion object {
        val EMPTY = CompilerPlugins()

        private val OPTION = Regex("^plugin:[^:=\\s]+:[^=\\s:]+=.*$", RegexOption.DOT_MATCHES_ALL)

        // Plugin set key -> "" when it loads, else why not. Shared by sessions, rebuilds and module compiles.
        private val probes = ConcurrentHashMap<String, String>()

        // Plugin sets whose left-out plugins were logged to stderr.
        private val logged: MutableSet<String> = ConcurrentHashMap.newKeySet()

        internal fun clearProbes() {
            probes.clear()
            logged.clear()
        }

        /** The probe failure of [plugins] ("" when it loads), computed once per plugin set per process. */
        private fun probeCached(plugins: CompilerPlugins, classpath: List<String>): String =
            probes.computeIfAbsent(plugins.addTo(FingerprintBuilder()).build()) { probe(plugins, classpath) }

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
                return errors.firstOrNull() ?: "compiler exited with $exit"
            } finally {
                dir.deleteRecursively()
            }
        }
    }
}
