package dev.jasonpearson.krit.fir

import java.io.File
import java.util.jar.JarFile

/** Scans the plugin code source and test classpath roots; no per-rule index or service file. */
object FirRuleDiscovery {
    private const val prefix = "dev/jasonpearson/krit/fir/checkers/"
    val rules: List<FirRule> by lazy(LazyThreadSafetyMode.SYNCHRONIZED) { discover() }

    fun enabled(context: FirRuleCompileContext? = FirRuleContext.current()): List<FirRule> =
        when {
            context?.noneEnabled == true -> emptyList()
            context == null || context.enabledRuleIds.isEmpty() -> rules
            else -> rules.filter { it.ruleId in context.enabledRuleIds }
        }

    private fun discover(): List<FirRule> {
        val roots = linkedSetOf<File>()
        // The production daemon's code source is its jar. Gradle tests also put
        // test-only rules in a separate classes directory on java.class.path.
        FirRuleDiscovery::class.java.protectionDomain?.codeSource?.location?.toURI()?.let { roots += File(it) }
        System.getProperty("java.class.path").split(File.pathSeparator).filter { it.isNotBlank() }
            .forEach { roots += File(it) }
        val loader = FirRuleDiscovery::class.java.classLoader
        // Gradle test workers use an isolated URLClassLoader whose roots are
        // absent from java.class.path; package resources expose those roots.
        val resources = loader.getResources(prefix)
        while (resources.hasMoreElements()) {
            val url = resources.nextElement()
            when (url.protocol) {
                "file" -> runCatching {
                    var root = File(url.toURI())
                    repeat(prefix.trimEnd('/').split('/').size) { root = root.parentFile }
                    roots += root
                }
                "jar" -> runCatching {
                    val raw = url.toString().removePrefix("jar:").substringBefore("!/")
                    roots += File(java.net.URI(raw))
                }
            }
        }
        val result = mutableListOf<FirRule>()
        val seenClasses = mutableSetOf<String>()
        for (root in roots) {
            val names = when {
                root.isDirectory -> File(root, prefix).takeIf { it.isDirectory }?.walkTopDown()
                    ?.filter { it.isFile && it.extension == "class" }
                    ?.map { it.relativeTo(root).path.replace(File.separatorChar, '/') }
                    ?.toList().orEmpty()
                root.isFile && root.extension == "jar" -> JarFile(root).use { jar ->
                    jar.entries().asSequence().map { it.name }
                        .filter { it.startsWith(prefix) && it.endsWith(".class") }.toList()
                }
                else -> emptyList()
            }
            for (path in names.sorted()) {
                if ('$' in path) continue
                val name = path.removeSuffix(".class").replace('/', '.')
                if (!seenClasses.add(name)) continue
                val cls = Class.forName(name, false, loader)
                if (!FirRule::class.java.isAssignableFrom(cls)) continue
                val instance = runCatching { cls.getField("INSTANCE").get(null) }.getOrNull()
                if (instance is FirRule) result += instance
            }
        }
        val byId = mutableMapOf<String, FirRule>()
        for (rule in result.sortedBy { it.ruleId }) {
            val old = byId.putIfAbsent(rule.ruleId, rule)
            require(old == null) {
                "Duplicate FIR rule id '${rule.ruleId}': ${old!!::class.java.name} and ${rule::class.java.name}"
            }
        }
        return result.sortedBy { it.ruleId }
    }
}
