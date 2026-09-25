package dev.jasonpearson.krit.fir.runner

import org.jetbrains.kotlin.cli.common.arguments.K2JVMCompilerArguments
import java.io.File

/**
 * Splits a Kotlin Multiplatform project's JVM-side sources into K2's two
 * compilation levels: common sources and platform sources.
 *
 * The Go launcher has already dropped roots of non-JVM target source sets
 * (js, wasm, native, apple, ...). What is left still mixes `expect`
 * declarations with their JVM `actual`s. Compiled as one flat JVM module,
 * every call through an `expect` becomes ambiguous and loses its type and
 * warnings. Marking the right files as `-Xcommon-sources` under
 * `-Xmulti-platform` gives the same result as the JVM compilation kotlinc
 * runs for a common + jvm project.
 *
 * K2's CLI model has exactly two levels, so each root is placed on one side,
 * matching kotlinc ground truth measured per shape:
 *  - `commonMain` / `commonTest` are common.
 *  - Any other `src/<sourceSet>/kotlin` root (an intermediate set such as
 *    `concurrentMain` or `jvmCommonMain`, and even a leaf) is common when its
 *    files declare an `expect` and declare no `actual`: its actuals live in a
 *    platform set, and compiling it as platform puts `expect` and `actual` in
 *    one module (errors, ambiguous calls). JVM-only APIs still resolve from
 *    the common level in a JVM compilation.
 *  - A root that declares any `actual` stays platform. Promoting it would
 *    pull the `actual` into the common module next to its `expect` and break
 *    common code. A root holding both an `actual` and a new `expect` cannot
 *    be modeled with two levels; leaving it platform keeps the damage inside
 *    that root instead of spreading to commonMain.
 *  - `main` / `test` and roots outside the `src/<sourceSet>/` layout are
 *    never promoted.
 *
 * Without a `commonMain`/`commonTest` root the project is not treated as
 * multiplatform at all and no file is read, so plain JVM and Android
 * projects compile exactly as before.
 */
internal object MultiplatformSources {

    private val commonSourceSets = setOf("commonMain", "commonTest")
    private val neverPromoted = setOf("main", "test")

    /**
     * Configures [args] for a multiplatform JVM compilation when [sourceDirs]
     * contain a common source set. [sourceFiles] must be the exact strings
     * placed in `freeArgs`: K2 only treats a file as common when its
     * `-Xcommon-sources` entry matches the free argument.
     */
    fun configure(args: K2JVMCompilerArguments, sourceDirs: List<String>, sourceFiles: List<String>) {
        val common = commonSources(sourceDirs, sourceFiles)
        if (common.isEmpty()) return
        args.multiPlatform = true
        args.expectActualClasses = true
        args.commonSources = common.toTypedArray()
    }

    /** The subset of [sourceFiles] K2 should compile as common sources. */
    fun commonSources(sourceDirs: List<String>, sourceFiles: List<String>): List<String> {
        val bySourceSet = sourceDirs.map { it to sourceSetName(it) }
        if (bySourceSet.none { (_, name) -> name in commonSourceSets }) return emptyList()
        val commonRoots = bySourceSet.mapNotNull { (dir, name) ->
            when {
                name == null || name in neverPromoted -> null
                name in commonSourceSets -> dir
                declaresExpectOnly(dir) -> dir
                else -> null
            }
        }
        if (commonRoots.isEmpty()) return emptyList()
        // Match by the root as passed (a walked file keeps that spelling even
        // when it is a symlink leading elsewhere) and by canonical path (a
        // requested file may be spelled differently from its root).
        val prefixes = commonRoots.flatMap {
            listOf(File(it).absoluteFile.normalize().path + File.separator, canonicalOrSelf(it) + File.separator)
        }.distinct()
        return sourceFiles.filter { path ->
            val absolute = File(path).absoluteFile.normalize().path
            val canonical = canonicalOrSelf(path)
            prefixes.any { absolute.startsWith(it) || canonical.startsWith(it) }
        }
    }

    /** `<name>` for a root shaped `.../src/<name>/kotlin` (or `java`), else null. */
    internal fun sourceSetName(dir: String): String? {
        val sourceSet = File(dir).absoluteFile.parentFile ?: return null
        if (sourceSet.parentFile?.name != "src") return null
        return sourceSet.name
    }

    private fun declaresExpectOnly(dir: String): Boolean {
        var sawExpect = false
        File(dir).walkTopDown().filter { it.isFile && it.extension == "kt" }.forEach { file ->
            val text = try { file.readText() } catch (_: Exception) { return@forEach }
            val code = stripCommentsAndStrings(text)
            if (ACTUAL_MODIFIER.containsMatchIn(code)) return false
            if (!sawExpect && EXPECT_MODIFIER.containsMatchIn(code)) sawExpect = true
        }
        return sawExpect
    }

    // `expect`/`actual` are soft keywords: `expect(value)`, `val actual = x`
    // and `assertEquals(expected, actual)` are ordinary identifiers. Only the
    // modifier form, followed on the same line by another modifier or a
    // declaration keyword, counts.
    private const val FOLLOWING =
        "(?:public|internal|private|protected|abstract|open|final|sealed|enum|annotation|data|value|" +
            "inline|external|suspend|infix|operator|tailrec|inner|override|const|lateinit|fun|class|" +
            "interface|object|val|var|typealias|constructor)\\b"
    private val EXPECT_MODIFIER = Regex("(?<![\\w.`])expect[ \\t]+$FOLLOWING")
    private val ACTUAL_MODIFIER = Regex("(?<![\\w.`])actual[ \\t]+$FOLLOWING")

    /**
     * Blanks out comments and string/char literals so words inside them are
     * not read as modifiers. String templates are blanked with the string;
     * a declaration never appears inside one.
     */
    internal fun stripCommentsAndStrings(text: String): String {
        val out = StringBuilder(text.length)
        var i = 0
        val n = text.length
        while (i < n) {
            val c = text[i]
            when {
                text.startsWith("//", i) -> {
                    while (i < n && text[i] != '\n') i++
                }
                text.startsWith("/*", i) -> {
                    // Kotlin block comments nest.
                    var depth = 0
                    while (i < n) {
                        if (text.startsWith("/*", i)) { depth++; i += 2; continue }
                        if (text.startsWith("*/", i)) { depth--; i += 2; if (depth == 0) break; continue }
                        if (text[i] == '\n') out.append('\n')
                        i++
                    }
                    out.append(' ')
                }
                text.startsWith("\"\"\"", i) -> {
                    val end = text.indexOf("\"\"\"", i + 3)
                    i = if (end < 0) n else end + 3
                    while (i < n && text[i] == '"') i++
                    out.append("\"\"")
                }
                c == '"' || c == '\'' -> {
                    i++
                    while (i < n && text[i] != c && text[i] != '\n') {
                        if (text[i] == '\\') i++
                        i++
                    }
                    i++
                    out.append(c).append(c)
                }
                else -> {
                    out.append(c)
                    i++
                }
            }
        }
        return out.toString()
    }

    private fun canonicalOrSelf(path: String): String =
        try { File(path).canonicalPath } catch (_: Exception) { File(path).absolutePath }
}
