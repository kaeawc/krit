package dev.jasonpearson.krit.gradle

import org.gradle.api.Action
import org.gradle.api.Project
import org.gradle.api.file.Directory
import org.gradle.api.file.FileCollection
import org.gradle.api.logging.Logging
import org.gradle.api.provider.Provider

/**
 * The Java and Kotlin source roots AGP's Variant API reports for each Android component
 * (`variant.sources.java.all` / `variant.sources.kotlin.all`), keyed by component name
 * (`debug`, `debugUnitTest`, `debugAndroidTest`).
 *
 * Those providers include the directories that tasks generate (view binding, BuildConfig,
 * AIDL, SQLDelight, Wire, KSP: any generator that registers its output through
 * `addGeneratedSourceDirectory` or `srcDir`) and carry the producing tasks as build
 * dependencies. That makes the model self-describing about generated roots, and lets
 * `exportGenerated` run exactly the generators the build registered, without knowing task
 * names.
 *
 * AGP runs `onVariants` callbacks while it creates variants, so they must be registered when
 * AGP is applied, before the plugin's own `afterEvaluate` export. The plugin has no compile
 * dependency on AGP, so the API is reached by reflection; on an AGP without it (before 8.0)
 * the map stays empty and the export falls back to the declared source directories.
 */
internal class AndroidVariantSources(private val project: Project) {
    private val roots = linkedMapOf<String, MutableList<Provider<out Collection<Directory>>>>()
    private var registered = false

    /** Registers the `onVariants` callback once; a no-op without `androidComponents`. */
    fun register() {
        if (registered) return
        val components = project.extensions.findByName("androidComponents") ?: return
        registered = true
        try {
            val selector = reflectCall(reflectCall(components, "selector"), "all")
            val onVariants = components.javaClass.methods.first {
                it.name == "onVariants" && it.parameterCount == 2 &&
                    Action::class.java.isAssignableFrom(it.parameterTypes[1])
            }
            onVariants.invoke(components, selector, Action<Any> { record(this) })
        } catch (error: Exception) {
            val cause = (error as? java.lang.reflect.InvocationTargetException)?.targetException ?: error
            project.logger.info("krit: Android Variant API sources unavailable for ${project.path} " +
                "(${cause.javaClass.simpleName}: ${cause.message}); exporting declared source directories only")
        }
    }

    private fun record(variant: Any) {
        val components = listOf(variant) + listOf("getUnitTest", "getAndroidTest", "getTestFixtures").mapNotNull { getter ->
            runCatching { variant.javaClass.getMethod(getter).invoke(variant) }.getOrNull()
        }
        components.forEach { component ->
            val name = runCatching { reflectCall(component, "getName").toString() }.getOrNull() ?: return@forEach
            val sources = runCatching { reflectCall(component, "getSources") }.getOrNull() ?: return@forEach
            listOf("getJava", "getKotlin").forEach { language ->
                // Sources.java / Sources.kotlin are null when the component has no such sources.
                val directories = runCatching { sources.javaClass.getMethod(language).invoke(sources) }.getOrNull()
                @Suppress("UNCHECKED_CAST")
                val all = directories?.let { runCatching { reflectCall(it, "getAll") }.getOrNull() }
                    as? Provider<out Collection<Directory>>
                if (all != null) roots.getOrPut(name) { mutableListOf() } += all
            }
        }
    }

    /** True when the Variant API reported roots for [component]. */
    fun has(component: String): Boolean = roots.containsKey(component)

    /**
     * The component's source roots as model paths, with no task dependency: reading them
     * resolves directory locations only, so exporting without `exportGenerated` runs no
     * generator.
     */
    fun paths(component: String): Provider<List<String>> {
        val providers = roots[component].orEmpty().toList()
        val projectPath = project.path
        return project.provider {
            providers.flatMap { provider ->
                // A plugin that registered a root derived from its task's output content (not
                // its location) cannot be read before that task runs; skip it rather than fail.
                runCatching { provider.get().map { modelPath(it.asFile) } }.getOrElse { error ->
                    Logging.getLogger(AndroidVariantSources::class.java).warn(
                        "krit: a generated source root of $projectPath $component is unknown until its task runs; " +
                            "it is not in the model (${error.message?.lineSequence()?.firstOrNull()})")
                    emptyList()
                }
            }.distinct().sorted()
        }
    }

    /** The same roots as a file collection that carries their producer tasks as build dependencies. */
    fun producers(component: String): FileCollection = project.files(*roots[component].orEmpty().toTypedArray())

    private fun reflectCall(value: Any, method: String): Any =
        value.javaClass.getMethod(method).invoke(value)
            ?: throw ReflectiveOperationException("$method returned null")
}
