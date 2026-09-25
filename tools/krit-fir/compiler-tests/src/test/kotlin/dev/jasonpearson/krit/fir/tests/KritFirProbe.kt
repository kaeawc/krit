package dev.jasonpearson.krit.fir.tests

import dev.jasonpearson.krit.fir.FirRuleCompileContext
import dev.jasonpearson.krit.fir.FirRuleContext
import org.jetbrains.kotlin.cli.common.ExitCode
import org.jetbrains.kotlin.cli.common.arguments.K2JVMCompilerArguments
import org.jetbrains.kotlin.cli.common.messages.CompilerMessageSeverity
import org.jetbrains.kotlin.cli.common.messages.CompilerMessageSourceLocation
import org.jetbrains.kotlin.cli.common.messages.MessageCollector
import org.jetbrains.kotlin.cli.jvm.K2JVMCompiler
import org.jetbrains.kotlin.config.Services
import java.io.File

// Compiles in-memory Kotlin sources (plus the shared stubs) with the krit-fir
// plugin and returns the diagnostics it emitted. Shared by AbstractDiagnosticTest
// (golden marker tests) and the property tests, so both exercise one compile path.
object KritFirProbe {

    data class Diag(val file: String, val line: Int, val name: String)

    private val stubsDir: File get() = File("src/test/data/stubs")

    // Outcome of one in-memory compile. [compileErrors] holds the non-plugin
    // ERROR diagnostics located in requested sources ("File.kt:line: message");
    // [otherErrors] holds every other ERROR: messages with no source location
    // (a missing source root, a bad plugin or classpath entry) and errors
    // located in the stubs. The compile is clean only
    // when K2 exits OK: KRIT_RULE is a warning, so findings never change the
    // exit code, while an error anywhere (including INTERNAL_ERROR from a
    // checker that threw, or an error inside a stub) does.
    data class Compilation(
        val diags: List<Diag>,
        val compileErrors: List<String>,
        val otherErrors: List<String>,
        val exitCode: ExitCode,
    ) {
        val crashed: Boolean get() = exitCode == ExitCode.INTERNAL_ERROR
        val clean: Boolean get() = exitCode == ExitCode.OK && compileErrors.isEmpty()

        // Why the compile is not clean, for failure messages.
        fun problems(): String = buildString {
            appendLine("exit=$exitCode")
            compileErrors.forEach { appendLine(it) }
            otherErrors.forEach { appendLine(it) }
            if (crashed) appendLine("(a checker threw; see the compiler exception above)")
            if (compileErrors.isEmpty() && otherErrors.isEmpty() && !crashed) {
                appendLine("(the compiler failed without reporting an error)")
            }
        }.trimEnd()
    }

    // Compiles all [sources] (keyed by filename) together and returns every krit
    // plugin diagnostic, tagged with its factory name (the [NAME] render prefix),
    // file name, and 1-based line. Fails when a requested source does not compile.
    // [ruleContext] limits the enabled rules; null enables every rule.
    fun diagnose(sources: Map<String, String>, ruleContext: FirRuleCompileContext? = null): List<Diag> {
        val result = compile(sources, ruleContext)
        // A non-plugin ERROR in a requested source means the snippet did not
        // compile. Without this, a checker that bails on unresolved symbols
        // yields "no diagnostics", making every negative case (and golden
        // negative) pass vacuously. A checker that throws makes K2 return
        // INTERNAL_ERROR without a requested-file ERROR line, so the exit code
        // catches crashes that the per-file error scan misses. Fail loudly.
        check(result.clean) {
            "Test snippet(s) did not compile cleanly — checker verdicts would be vacuous:\n" +
                result.problems().prependIndent("  ")
        }
        return result.diags
    }

    // Compiles [sources] like [diagnose] but reports compile errors instead of
    // failing on them. [ruleContext] selects the FIR rules (and their options)
    // exactly as a production check request does; null enables every rule.
    // [testFiles] names the [sources] keys the request classifies as test
    // files (FirRule.isTestFile); it needs a [ruleContext]. A key may contain
    // `/` to place the source in a subdirectory.
    // [configure] adjusts the compiler arguments (tests of the probe itself).
    fun compile(
        sources: Map<String, String>,
        ruleContext: FirRuleCompileContext? = null,
        testFiles: Set<String> = emptySet(),
        configure: (K2JVMCompilerArguments) -> Unit = {},
    ): Compilation {
        require(testFiles.isEmpty() || ruleContext != null) { "testFiles needs a ruleContext" }
        require(sources.keys.containsAll(testFiles)) { "testFiles must name sources: $testFiles" }
        val pluginJar = requireNotNull(locatePluginJar()) {
            "krit-fir plugin JAR not found. Set 'krit.fir.plugin.jar' or run `./gradlew :jar`."
        }
        val tmpDir = kotlin.io.path.createTempDirectory("krit-fir-probe").toFile()
        try {
            // Kotlin sources (requested + Kotlin stubs) and the Java stub layer
            // live in sibling roots so the Java files keep their package
            // directories and are resolved as Java sources, not Kotlin ones.
            val ktDir = tmpDir.resolve("src").apply { mkdirs() }
            val javaDir = tmpDir.resolve("java").apply { mkdirs() }
            sources.forEach { (name, body) -> ktDir.resolve(name).apply { parentFile.mkdirs() }.writeText(body) }
            if (stubsDir.isDirectory) {
                stubsDir.listFiles { f -> f.extension == "kt" }?.forEach { stub ->
                    stub.copyTo(ktDir.resolve(stub.name), overwrite = true)
                }
            }
            val javaStubsDir = stubsDir.resolve("java")
            if (javaStubsDir.isDirectory) {
                javaStubsDir.walkTopDown().filter { it.isFile && it.extension == "java" }.forEach { stub ->
                    stub.copyTo(javaDir.resolve(stub.relativeTo(javaStubsDir)), overwrite = true)
                }
            }
            val outDir = tmpDir.resolve("out").apply { mkdirs() }
            val diags = mutableListOf<Diag>()
            val compileErrors = mutableListOf<String>()
            val otherErrors = mutableListOf<String>()
            val requested = sources.keys.mapTo(HashSet()) { File(it).name }
            val collector = object : MessageCollector {
                override fun clear() {}
                override fun hasErrors() = false
                override fun report(
                    severity: CompilerMessageSeverity,
                    message: String,
                    location: CompilerMessageSourceLocation?,
                ) {
                    if (location == null) {
                        if (severity == CompilerMessageSeverity.ERROR) otherErrors.add("(no location): $message")
                        return
                    }
                    val fileName = File(location.path).name
                    val match = pluginDiagnosticRe.find(message)
                    if (match != null) {
                        if (severity in reportable) diags.add(Diag(fileName, location.line, match.groupValues[1]))
                        return
                    }
                    if (severity == CompilerMessageSeverity.ERROR) {
                        val located = "$fileName:${location.line}: $message"
                        if (fileName in requested) compileErrors.add(located) else otherErrors.add(located)
                    }
                }
            }
            val stdlibJar = System.getProperty("kotlin.stdlib.jar")?.let { File(it).takeIf { f -> f.exists() } }
            // The plugin jar is loaded by a child of this class loader, so the
            // plugin sees the same FirRuleContext object the test sets here
            // (K2 builds its checkers on the calling thread).
            if (ruleContext != null) {
                val paths = testFiles.map { ktDir.resolve(it).absolutePath }
                FirRuleContext.begin(ruleContext.copy(testFiles = ruleContext.testFiles + paths))
            }
            val exitCode = try {
                K2JVMCompiler().exec(
                    collector,
                    Services.EMPTY,
                    K2JVMCompilerArguments().apply {
                        freeArgs = listOf(ktDir.absolutePath)
                        // K2 resolves Java sources directly (no javac), giving the
                        // Android platform stubs real Java symbol shapes: statics,
                        // synthetic properties, and platform types.
                        javaSourceRoots = arrayOf(javaDir.absolutePath)
                        destination = outDir.absolutePath
                        noStdlib = true
                        noReflect = true
                        if (stdlibJar != null) classpath = stdlibJar.absolutePath
                        pluginClasspaths = arrayOf(pluginJar.absolutePath)
                        configure(this)
                    },
                )
            } finally {
                if (ruleContext != null) FirRuleContext.end()
            }
            return Compilation(diags, compileErrors, otherErrors, exitCode)
        } finally {
            tmpDir.deleteRecursively()
        }
    }

    // Diagnostics for one source file (named "Main.kt"), the common single-file case.
    fun diagnose(source: String): List<Diag> = diagnose(mapOf("Main.kt" to source))

    private fun locatePluginJar(): File? {
        System.getProperty("krit.fir.plugin.jar")?.let { return File(it).takeIf { f -> f.exists() } }
        return File("../build/libs")
            .takeIf { it.isDirectory }
            ?.listFiles { f -> f.name.startsWith("krit-fir") && f.name.endsWith(".jar") }
            ?.firstOrNull()
    }

    private val reportable = setOf(
        CompilerMessageSeverity.WARNING,
        CompilerMessageSeverity.STRONG_WARNING,
        CompilerMessageSeverity.ERROR,
    )
    private val pluginDiagnosticRe = Regex("""^\[([A-Za-z][A-Za-z0-9_]*)]""")
}
