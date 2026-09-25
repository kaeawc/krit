package dev.jasonpearson.krit.fir

import java.io.File
import java.lang.reflect.Modifier
import java.util.jar.JarFile

/** Scans the plugin code source and test classpath roots; no per-rule index or service file. */
object FirRuleDiscovery {
    internal const val CHECKERS_PREFIX = "dev/jasonpearson/krit/fir/checkers/"
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
        val resources = loader.getResources(CHECKERS_PREFIX)
        while (resources.hasMoreElements()) {
            val url = resources.nextElement()
            when (url.protocol) {
                "file" -> runCatching {
                    var root = File(url.toURI())
                    repeat(CHECKERS_PREFIX.trimEnd('/').split('/').size) { root = root.parentFile }
                    roots += root
                }
                "jar" -> runCatching {
                    val raw = url.toString().removePrefix("jar:").substringBefore("!/")
                    roots += File(java.net.URI(raw))
                }
            }
        }
        return discover(roots, loader)
    }

    /**
     * Scans [roots] (class directories or jars) for rule objects under [prefix].
     *
     * Every concrete [FirRule] implementer found must be a Kotlin `object`
     * (top-level or nested, any visibility); anything else is an authoring
     * error and fails discovery loudly rather than silently dropping the rule.
     */
    internal fun discover(
        roots: Collection<File>,
        loader: ClassLoader,
        prefix: String = CHECKERS_PREFIX,
    ): List<FirRule> {
        val result = mutableListOf<FirRule>()
        val seenClasses = mutableSetOf<String>()
        for (root in roots) {
            for (path in classEntries(root, prefix).sorted()) {
                val name = path.removeSuffix(".class").replace('/', '.')
                if (!seenClasses.add(name)) continue
                val cls = Class.forName(name, false, loader)
                if (!FirRule::class.java.isAssignableFrom(cls)) continue
                if (cls.isInterface || Modifier.isAbstract(cls.modifiers)) continue
                // Anonymous/local classes (e.g. an `object : FirRule` expression
                // inside a helper) are never rule declarations.
                if (cls.isAnonymousClass || cls.isLocalClass || cls.isSynthetic) continue
                result += ruleInstance(cls)
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

    private fun classEntries(root: File, prefix: String): List<String> = when {
        root.isDirectory -> File(root, prefix).takeIf { it.isDirectory }?.walkTopDown()
            ?.filter { it.isFile && it.extension == "class" }
            ?.map { it.relativeTo(root).path.replace(File.separatorChar, '/') }
            ?.toList().orEmpty()
        // Roots include the whole JVM classpath, so one unreadable jar must
        // not take down discovery (and with it every check request).
        root.isFile && root.extension == "jar" -> try {
            JarFile(root).use { jar ->
                jar.entries().asSequence().map { it.name }
                    .filter { it.startsWith(prefix) && it.endsWith(".class") }.toList()
            }
        } catch (e: Exception) {
            System.err.println("krit-fir: skipping unreadable classpath jar ${root.path}: ${e.message}")
            emptyList()
        }
        else -> emptyList()
    }

    private fun ruleInstance(cls: Class<*>): FirRule {
        // getDeclaredField + setAccessible: a file-private top-level object
        // compiles to a package-private class whose INSTANCE is otherwise
        // inaccessible from this package.
        val instance = try {
            cls.getDeclaredField("INSTANCE").apply { isAccessible = true }.get(null)
        } catch (e: Exception) {
            error(
                "FIR rule class ${cls.name} implements FirRule but is not a readable Kotlin object " +
                    "(${e.javaClass.simpleName}: ${e.message}). Declare it as `object ${cls.simpleName} : FirRule`.",
            )
        }
        return instance as? FirRule
            ?: error("FIR rule class ${cls.name}: INSTANCE is ${instance?.javaClass?.name}, not a FirRule")
    }
}
