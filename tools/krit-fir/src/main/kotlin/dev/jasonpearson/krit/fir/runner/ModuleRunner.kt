package dev.jasonpearson.krit.fir.runner

import org.jetbrains.kotlin.cli.common.arguments.K2JVMCompilerArguments
import org.jetbrains.kotlin.cli.common.arguments.parseCommandLineArguments
import java.io.File
import java.nio.file.Files
import java.nio.file.StandardCopyOption
import java.security.MessageDigest

data class ModuleFragment(val name: String, val sourceRoots: List<String>, val refines: List<String> = emptyList())

data class ModuleSpec(
    val id: String,
    val platform: String = "jvm",
    val kind: String = "main",
    val sourceRoots: List<String> = emptyList(),
    val generatedSourceRoots: List<String> = emptyList(),
    val classpath: List<String> = emptyList(),
    val dependsOn: List<String> = emptyList(),
    val friends: List<String> = emptyList(),
    val compilerArgs: List<String> = emptyList(),
    val jvmTarget: String = "1.8",
    val fragments: List<ModuleFragment> = emptyList(),
) {
    internal val edges get() = (dependsOn + friends).distinct()
    internal val roots get() = (sourceRoots + generatedSourceRoots + fragments.flatMap { it.sourceRoots }).distinct()
}

data class ModuleStatus(val id: String, val mode: String, val firstError: String? = null)

/** Only the module driver supplies these; legacy compiles retain their existing defaults. */
internal data class ModuleCompilation(
    val sources: List<String>,
    val output: File,
    val classpath: List<String>,
    val friends: List<String>,
    val spec: ModuleSpec,
    val extraArgs: List<String>,
    val legacy: Boolean = false,
) {
    fun configure(args: K2JVMCompilerArguments) {
        args.classpath = effectiveClasspath(classpath).joinToString(File.pathSeparator)
        args.friendPaths = friends.toTypedArray()
        args.jvmTarget = spec.jvmTarget
        // Module IDs can contain ':' and other filesystem-unsafe characters.
        args.moduleName = "krit_" + digest(spec.id)
        if (legacy || spec.fragments.isEmpty()) {
            MultiplatformSources.configure(args, spec.roots, sources)
        } else {
            // Verified against kotlin-compiler 2.3.21 CommonCompilerArguments annotations.
            // Use one mapping per FILE (not a directory); spelling must match freeArgs.
            val flags = mutableListOf("-Xmulti-platform", "-Xfragments=" + spec.fragments.joinToString(",") { it.name })
            val refined = spec.fragments.flatMap { it.refines }.toSet()
            val leaves = spec.fragments.filter { it.name !in refined }.sortedBy { it.name }
            val leaf = leaves.singleOrNull()
                ?: leaves.firstOrNull { it.name.contains(spec.platform, ignoreCase = true) }
                ?: leaves.firstOrNull() ?: spec.fragments.sortedBy { it.name }.firstOrNull()
            val owner = sources.associateWith { source ->
                spec.fragments.firstOrNull { inRoots(source, it.sourceRoots) } ?: leaf ?: error("Unowned module source: $source")
            }
            for (fragment in spec.fragments) {
                for (source in sources.filter { owner[it] == fragment }) {
                    flags += "-Xfragment-sources=${fragment.name}:$source"
                }
                for (parent in fragment.refines) flags += "-Xfragment-refines=${fragment.name}:$parent"
            }
            parseCommandLineArguments(flags, args)
        }
        parseCommandLineArguments(extraArgs, args)
    }
}

/** Remove managed options in both split and equals forms, including their operands. */
internal fun filterModuleArgs(input: List<String>): List<String> {
    val out = mutableListOf<String>()
    var i = 0
    val valued = setOf("-d", "-classpath", "-cp", "-jvm-target", "-Xplugin")
    while (i < input.size) {
        val arg = input[i++]
        val key = arg.substringBefore('=')
        when {
            key == "-Werror" || key in valued -> {
                System.err.println("krit-fir: ignoring managed compiler argument $arg")
                if (arg == key && key in valued && i < input.size) i++
            }
            arg == "-P" && input.getOrNull(i)?.startsWith("plugin:") == true -> {
                System.err.println("krit-fir: ignoring plugin compiler argument ${input[i]}")
                i++
            }
            arg.startsWith("-P=plugin:") || arg.startsWith("-P plugin:") ->
                System.err.println("krit-fir: ignoring plugin compiler argument $arg")
            else -> out += arg
        }
    }
    return out
}

internal fun inRoots(path: String, roots: List<String>): Boolean {
    val file = File(path).canonicalFile.toPath()
    return roots.any { file.startsWith(File(it).canonicalFile.toPath()) }
}

internal fun moduleSources(spec: ModuleSpec): List<String> = spec.roots.flatMap { root ->
    File(root).walkTopDown().filter { it.isFile && it.extension in setOf("kt", "java") }
        .map { it.canonicalPath }.toList()
}.distinct().sorted()

private fun digest(text: String): String = MessageDigest.getInstance("SHA-256")
    .digest(text.toByteArray()).joinToString("") { "%02x".format(it) }

/** Preserve upstream HMPP source ownership when its sources must replace missing binaries. */
private fun fallbackFragments(primary: ModuleSpec, merged: List<ModuleSpec>): List<ModuleFragment> {
    if (merged.none { it.fragments.isNotEmpty() }) return primary.fragments
    val own = primary.fragments.ifEmpty { listOf(ModuleFragment("main", primary.roots)) }
    val upstream = merged.flatMap { module ->
        val prefix = "upstream_${digest(module.id).take(16)}_"
        module.fragments.map { it.copy(name = prefix + it.name, refines = it.refines.map { parent -> prefix + parent }) }
    }
    val refined = upstream.flatMap { it.refines }.toSet()
    val upstreamLeaves = upstream.filter { it.name !in refined }.map { it.name }
    val ownRefined = own.flatMap { it.refines }.toSet()
    return upstream + own.map {
        if (it.name !in ownRefined) it.copy(refines = it.refines + upstreamLeaves) else it
    }
}

/** Tarjan emits dependency components before their consumers, including friend edges. */
internal fun moduleComponents(modules: List<ModuleSpec>): List<List<ModuleSpec>> {
    val byId = modules.associateBy { it.id }
    require(byId.size == modules.size) { "Duplicate module id" }
    for (module in modules) {
        require(module.platform in setOf("jvm", "android")) { "Unsupported platform: ${module.platform}" }
        for (edge in module.edges) require(edge in byId) { "${module.id}: unknown module $edge" }
    }
    val indices = mutableMapOf<String, Int>()
    val low = mutableMapOf<String, Int>()
    val stack = mutableListOf<String>()
    val active = mutableSetOf<String>()
    val result = mutableListOf<List<ModuleSpec>>()
    fun visit(id: String) {
        indices[id] = indices.size
        low[id] = indices.getValue(id)
        stack += id
        active += id
        for (edge in byId.getValue(id).edges) {
            if (edge !in indices) {
                visit(edge)
                low[id] = minOf(low.getValue(id), low.getValue(edge))
            } else if (edge in active) low[id] = minOf(low.getValue(id), indices.getValue(edge))
        }
        if (low[id] == indices[id]) {
            val component = mutableListOf<ModuleSpec>()
            do {
                val member = stack.removeAt(stack.lastIndex)
                active -= member
                component += byId.getValue(member)
            } while (member != id)
            result += component.reversed()
        }
    }
    modules.forEach { if (it.id !in indices) visit(it.id) }
    return result
}

/** Session-owned outputs and verdicts. Deliberately synchronous: the Kotlin core environment is global. */
internal class ModuleRunner(
    // Bound repeated recovery work; larger subgraphs use one explicit legacy union.
    internal var mergeFileLimit: Int = 300,
    internal var beforeCompile: (ModuleCompilation) -> Unit = {},
) {
    private val directoryState = lazy { run {
        sweepAbandonedModuleDirectories(File(System.getProperty("java.io.tmpdir")))
        Files.createTempDirectory("krit-fir-modules-${ProcessHandle.current().pid()}-").toFile()
    } }
    private val directory by directoryState
    private data class Cached(val hash: String, val findingsKey: String, val result: BatchResult, val output: File, val gated: Boolean, val missingBinaries: Boolean)
    private val cache = mutableMapOf<String, Cached>()
    // A cached result owns its output for its entire lifetime, including cache hits.
    // Both dependsOn and friends consumers need these binaries on later requests;
    // gated outputs also keep their directory for merged-fallback compilation.
    internal val outputDirectories: Map<String, File> get() = cache.mapValues { it.value.output }
    internal val compiledSources = mutableMapOf<String, List<String>>()
    private data class FileHash(val size: Long, val modified: java.nio.file.attribute.FileTime, val hash: String)
    private val classpathHashes = mutableMapOf<String, FileHash>()
    internal var classpathHashReads = 0
        private set
    val compilationCounts = mutableMapOf<String, Int>()

    fun dispose() {
        cache.clear()
        if (directoryState.isInitialized()) directory.deleteRecursively()
    }

    fun check(
        id: Long, modules: List<ModuleSpec>, checkFiles: List<String>, rules: Set<String>,
        configs: Map<String, Map<String, Any?>>, testFiles: Set<String>, scanPaths: Map<String, String>,
    ): BatchResult {
        val components = moduleComponents(modules)
        val byId = modules.associateBy { it.id }
        val sources = modules.associate { it.id to moduleSources(it) }
        val args = modules.associate { it.id to filterModuleArgs(it.compilerArgs) }
        val orderedIds = components.flatten().map { it.id }
        val results = mutableListOf<BatchResult>()
        val statuses = mutableListOf<ModuleStatus>()
        val deciding = linkedMapOf<String, String>()
        val requested = checkFiles.distinct()
        // Shared common sources may participate in multiple target compilations.
        // Prefer the deepest root; declaration order only breaks equal-root ties.
        fun owner(path: String) = modules.mapNotNull { module ->
            module.roots.filter { inRoots(path, listOf(it)) }.maxOfOrNull { File(it).canonicalPath.length }
                ?.let { module.id to it }
        }.maxByOrNull { it.second }?.first
        val owners = requested.associateWith(::owner)
        val unknown = owners.filterValues { it == null }.keys
        for (component in components) {
            val ids = component.map { it.id }.toSet()
            val cycle = component.size > 1 || component.single().id in component.single().edges
            val reachable = linkedSetOf<String>()
            fun reach(moduleId: String) {
                for (edge in byId.getValue(moduleId).edges) if (edge !in ids && reachable.add(edge)) reach(edge)
            }
            ids.forEach { reach(it) }
            // K2 suppresses ALL backend output on any frontend error. Clean owners are
            // not gated, but their unavailable binaries still need source replacement.
            // Reconstruct from graph IDs, never recursively concatenate cached source lists.
            val merged = if (cycle) emptyList() else orderedIds.filter {
                it in reachable && cache[it]?.missingBinaries == true
            }.map { byId.getValue(it) }
            val capped = merged.sumOf { sources.getValue(it.id).size } > mergeFileLimit
            val ownFiles = requested.filter { owners[it] in ids }
            val key = digest(component.joinToString("|") { digest(it.id) })
            val hash = digest(inputHash(component, sources, args) +
                orderedIds.filter { it in reachable }.joinToString("") { cache.getValue(it).hash })
            val relevant = ownFiles.map { File(it).canonicalPath }.toSet()
            val findingsKey = digest(hash + "legacy=$cycle,$capped" + contextHash(ownFiles, rules, configs,
                testFiles.filter { File(it).canonicalPath in relevant }.sorted(),
                scanPaths.filterKeys { File(it).canonicalPath in relevant }))
            val old = cache[component.first().id]
            val dirty = old == null || old.hash != hash || old.findingsKey != findingsKey || !old.output.isDirectory
            val cached = if (!dirty) old else {
                ids.forEach { compilationCounts[it] = (compilationCounts[it] ?: 0) + 1 }
                val output = Files.createTempDirectory(directory.toPath(), "pending-").toFile()
                try {
                    val compiling = component + merged
                    val spec = component.first().copy(
                        sourceRoots = compiling.flatMap { it.roots },
                        generatedSourceRoots = emptyList(),
                        fragments = if (cycle) emptyList() else fallbackFragments(component.first(), merged),
                    )
                    val paths = compiling.flatMap { sources.getValue(it.id) }.distinct().toMutableList()
                    // Honor request spellings just as the legacy path does.
                    for (file in ownFiles) {
                        val canonical = File(file).canonicalPath
                        val index = paths.indexOf(canonical)
                        if (index >= 0) paths[index] = file
                        else if (file.endsWith(".kt")) paths += file
                    }
                    val friends = component.flatMap { it.friends }.filter { it !in ids }.distinct()
                    val cp = component.flatMap { it.classpath } + orderedIds.filter { it in reachable }
                        .mapNotNull { cache[it]?.output?.path }.distinct() + merged.flatMap { it.classpath }
                    val invocation = ModuleCompilation(paths, output, cp, friends.mapNotNull { cache[it]?.output?.path },
                        spec, args.getValue(component.first().id), legacy = cycle || capped)
                    ids.forEach { compiledSources[it] = paths.toList() }
                    beforeCompile(invocation)
                    val session = AnalysisSession(spec.roots, cp)
                    val ownedSources = component.flatMap { sources.getValue(it.id) }.filter { owner(it) in ids }.toSet()
                    val result = session.checkCompilation(id, ownFiles.map { FileRef(it) }, rules, configs, testFiles, scanPaths,
                        invocation, ownedSources)
                    check(!result.compilerCrashed) { result.firstCompilerError ?: "Module compiler crashed" }
                    // Publish a new generation, never clear a directory a cached entry references.
                    val published = File(directory, "$key-${java.util.UUID.randomUUID()}")
                    Files.move(output.toPath(), published.toPath(), StandardCopyOption.ATOMIC_MOVE)
                    Cached(hash, findingsKey, result, published, result.ownedCompilerError != null,
                        result.firstCompilerError != null)
                } catch (t: Throwable) {
                    ids.forEach { cache.remove(it) }
                    output.deleteRecursively()
                    reconcileOutputs()
                    throw t
                }
            }
            ids.forEach { cache[it] = cached }
            results += cached.result
            for (module in component) {
                val reason = if (cycle) {
                    "Dependency cycle: ${ids.joinToString(" -> ")}; used legacy union compilation" +
                        cached.result.firstCompilerError?.let { "; $it" }.orEmpty()
                } else if (capped) "Merge file limit $mergeFileLimit exceeded; used legacy union compilation"
                else cached.result.ownedCompilerError
                val mode = when {
                    cycle -> "skipped"
                    capped -> "legacy-fallback"
                    cached.gated -> "gated"
                    merged.isNotEmpty() -> "merged-fallback"
                    else -> "module"
                }
                statuses += ModuleStatus(module.id, mode, reason)
                ownFiles.filter { owners[it] == module.id }.forEach { deciding[it] = module.id }
            }
        }
        // Removed modules cannot supply stale outputs on a later request.
        cache.keys.retainAll(byId.keys)
        reconcileOutputs()
        val ruleErrors = linkedMapOf<String, MutableMap<String, String>>()
        results.forEach { r -> r.ruleErrors.forEach { (rule, errors) -> ruleErrors.getOrPut(rule) { linkedMapOf() }.putAll(errors) } }
        return BatchResult(id, results.sumOf { it.succeeded }, results.sumOf { it.skipped } + unknown.size,
            results.flatMap { it.findings }, results.flatMap { it.crashed.entries }.associate { it.toPair() },
            results.flatMap { it.rules }.distinct(),
            results.flatMap { it.errorFiles.entries }.associate { it.toPair() } + unknown.associateWith { "Not owned by any module" },
            ruleErrors, statuses, deciding)
    }

    private fun reconcileOutputs() {
        val live = cache.values.map { it.output }.toSet()
        if (directoryState.isInitialized()) directory.listFiles()?.filter { it !in live }?.forEach { it.deleteRecursively() }
    }

    private fun contextHash(files: List<String>, rules: Set<String>, configs: Map<String, Map<String, Any?>>,
                            tests: List<String>, paths: Map<String, String>): String {
        // Length framing prevents paths/config values containing delimiters from colliding.
        fun encode(value: Any?): String {
            val payload = when (value) {
                is Map<*, *> -> value.entries.sortedBy { it.key.toString() }
                    .joinToString("") { encode(it.key) + encode(it.value) }
                is Iterable<*> -> value.joinToString("") { encode(it) }
                else -> value.toString()
            }
            val kind = when (value) {
                is Map<*, *> -> "map"
                is Iterable<*> -> "list"
                else -> value?.javaClass?.name ?: "null"
            }
            return "$kind:${payload.length}:$payload"
        }
        return encode(listOf(files, rules.sorted(), configs, tests, paths))
    }

    private fun inputHash(
        modules: List<ModuleSpec>, sources: Map<String, List<String>>, args: Map<String, List<String>>,
    ): String {
        val hash = MessageDigest.getInstance("SHA-256")
        fun add(value: String) {
            val bytes = value.toByteArray()
            hash.update(bytes.size.toString().toByteArray())
            hash.update(0.toByte())
            hash.update(bytes)
        }
        fun value(item: Any?) {
            when (item) {
                null -> add("null")
                is Map<*, *> -> {
                    add("map"); add(item.size.toString())
                    item.forEach { (key, v) -> value(key); value(v) }
                }
                is Iterable<*> -> {
                    add("list"); add(item.count().toString())
                    item.forEach { value(it) }
                }
                else -> { add(item.javaClass.name); add(item.toString()) }
            }
        }
        fun content(file: File, memoize: Boolean = false) {
            add(file.canonicalPath)
            if (!file.exists()) { add("missing"); return }
            if (file.isDirectory) {
                file.walkTopDown().filter { it.isFile }.sortedBy { it.path }.forEach { content(it, memoize) }
            } else {
                add(file.length().toString())
                val modified = Files.getLastModifiedTime(file.toPath())
                val prior = classpathHashes[file.canonicalPath]
                val bytesHash = if (memoize && prior?.size == file.length() && prior.modified == modified) prior.hash else {
                    if (memoize) classpathHashReads++
                    val bytes = MessageDigest.getInstance("SHA-256")
                    file.inputStream().use { stream ->
                        val buffer = ByteArray(65536)
                        while (true) {
                            val count = stream.read(buffer)
                            if (count < 0) break
                            bytes.update(buffer, 0, count)
                        }
                    }
                    bytes.digest().joinToString("") { "%02x".format(it) }.also {
                        if (memoize) classpathHashes[file.canonicalPath] = FileHash(file.length(), modified, it)
                    }
                }
                add(bytesHash)
            }
        }
        for (module in modules) {
            value(listOf(module.id, module.platform, module.kind, module.sourceRoots, module.generatedSourceRoots,
                module.classpath, module.dependsOn, module.friends, args.getValue(module.id), module.jvmTarget,
                module.fragments.map { listOf(it.name, it.sourceRoots, it.refines) }))
            sources.getValue(module.id).forEach { content(File(it)) }
            module.classpath.forEach { content(File(it), true) }
        }
        return hash.digest().joinToString("") { "%02x".format(it) }
    }
}

/** Unknown/old naming schemes are deliberately left alone: ownership cannot be proved. */
internal fun sweepAbandonedModuleDirectories(parent: File, now: Long = System.currentTimeMillis()) {
    val pattern = Regex("krit-fir-modules-([0-9]+)-.+")
    parent.listFiles()?.forEach { dir ->
        val pid = pattern.matchEntire(dir.name)?.groupValues?.get(1)?.toLongOrNull() ?: return@forEach
        if (dir.isDirectory && now - dir.lastModified() > 24 * 60 * 60 * 1000L &&
            !ProcessHandle.of(pid).map { it.isAlive }.orElse(false)) dir.deleteRecursively()
    }
}
