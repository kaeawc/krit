package dev.jasonpearson.krit.gradle

import org.gradle.api.JavaVersion
import org.gradle.api.Plugin
import org.gradle.api.Project
import org.gradle.api.attributes.Category
import org.gradle.api.attributes.LibraryElements
import org.gradle.api.artifacts.component.ComponentIdentifier
import org.gradle.api.artifacts.component.ProjectComponentIdentifier
import org.gradle.api.artifacts.result.ResolvedDependencyResult
import org.gradle.api.artifacts.type.ArtifactTypeDefinition
import org.gradle.api.plugins.JavaPlugin
import org.gradle.api.tasks.SourceSetContainer
import org.gradle.api.provider.Provider
import org.gradle.api.file.RegularFile
import org.gradle.api.artifacts.Configuration
import org.gradle.api.file.FileCollection
import org.gradle.api.tasks.TaskProvider
import org.gradle.api.tasks.compile.JavaCompile
import org.gradle.jvm.toolchain.JavaLanguageVersion
import org.gradle.jvm.toolchain.JavaLauncher
import org.gradle.jvm.toolchain.JavaToolchainService
import java.io.File

/**
 * Gradle plugin that integrates krit Kotlin static analysis into the build.
 *
 * Registers the `krit` extension and the `kritCheck` task. The task downloads
 * the krit Go binary (if not cached) and invokes it as an external process,
 * producing reports in configured formats.
 *
 * Supports per-source-set tasks when the Android Gradle Plugin or Kotlin JVM
 * plugin is applied (e.g., kritCheckMain, kritCheckTest, kritCheckDebug).
 *
 * Usage:
 * ```
 * plugins {
 *     id("dev.jasonpearson.krit") version "<krit-version>"
 * }
 *
 * krit {
 *     advanced { toolVersion.set("<krit-version>") } // defaults to the plugin's version
 *     config.set(file("krit.yml"))
 *     reports {
 *         sarif { required.set(true) }
 *         json { required.set(false) }
 *     }
 * }
 * ```
 */
class KritPlugin : Plugin<Project> {

    override fun apply(project: Project) {
        project.pluginManager

        val extension = project.extensions.create("krit", KritExtension::class.java)

        // Resolvable configuration for declaring custom-rule producers as
        // project dependencies, e.g. `dependencies { kritCustomRules(project(":rules")) }`.
        // Matches the outgoing variant published by `dev.jasonpearson.krit.custom`,
        // so the stamped `kritRuleJar` archive flows through Gradle's dependency
        // graph (proper task wiring, no cross-project `evaluationDependsOn`).
        val kritRuleBundleCategory = project.objects.named(
            Category::class.java,
            KRIT_RULE_BUNDLE_CATEGORY,
        )
        val customRulesConfiguration = project.configurations.create("kritCustomRules") {
            isCanBeConsumed = false
            isCanBeResolved = true
            description = "Krit custom-rule bundles to load into the kritCheck analysis."
            attributes.attribute(Category.CATEGORY_ATTRIBUTE, kritRuleBundleCategory)
        }

        // Fold resolved bundles from `kritCustomRules` into the extension's
        // jar collection so they flow into kritCheck/kritBaseline like any
        // explicit `customRuleJars.from(file(...))` entry.
        extension.customRuleJars.from(customRulesConfiguration)

        // Set conventions (defaults)
        extension.ignoreFailures.convention(false)
        extension.exportModel.convention(true)
        extension.fir.convention(true)
        extension.exportGenerated.convention(false)
        extension.exportTests.convention(true)
        extension.androidVariant.convention("debug")
        extension.advanced.toolVersion.convention(KRIT_DEFAULT_VERSION)
        extension.advanced.allRules.convention(false)
        extension.advanced.fixLevel.convention("idiomatic")
        extension.advanced.parallel.convention(Runtime.getRuntime().availableProcessors())
        extension.advanced.noCache.convention(false)
        extension.advanced.typeInference.convention(true)
        extension.advanced.source.setFrom("src/main/kotlin", "src/test/kotlin")
        // A Kotlin Multiplatform project has no src/main/kotlin: its sources are its Kotlin source
        // sets' roots (src/commonMain/kotlin, src/jvmMain/kotlin, ...), read once they are configured
        // (#1752). A source the build already set is kept.
        project.pluginManager.withPlugin(KOTLIN_MULTIPLATFORM_PLUGIN_ID) {
            if (extension.advanced.source.from.toList() == DEFAULT_SOURCE) {
                extension.advanced.source.setFrom(project.provider { kmpSourceDirs(project) })
            }
        }
        extension.advanced.reportsDir.convention(
            project.layout.buildDirectory.dir("reports/krit")
        )

        // Set report conventions
        val reportsDir = extension.advanced.reportsDir
        extension.reports.sarif.required.convention(true)
        extension.reports.sarif.outputLocation.convention(
            reportsDir.map { it.file("krit.sarif") }
        )
        extension.reports.json.required.convention(false)
        extension.reports.json.outputLocation.convention(
            reportsDir.map { it.file("krit.json") }
        )
        extension.reports.plain.required.convention(false)
        extension.reports.plain.outputLocation.convention(
            reportsDir.map { it.file("krit.txt") }
        )
        extension.reports.checkstyle.required.convention(false)
        extension.reports.checkstyle.outputLocation.convention(
            reportsDir.map { it.file("krit-checkstyle.xml") }
        )

        // Register the binary resolver as a shared build service
        val binaryResolver = project.gradle.sharedServices.registerIfAbsent(
            BINARY_RESOLVER_SERVICE,
            KritBinaryResolver::class.java,
        ) {
            parameters.version.set(extension.advanced.toolVersion)
            parameters.cacheDir.set(
                project.layout.dir(
                    project.provider {
                        File(System.getProperty("user.home"), ".gradle/krit")
                    }
                )
            )
        }

        val advanced = extension.advanced
        // With an explicit binary the tool version is not used, so it must not be an
        // input: bumping the plugin would otherwise rerun every task.
        val toolVersionInput = advanced.binary.map { "" }.orElse(advanced.toolVersion)

        // krit-fir jar: KRIT_FIR_JAR when set (a local or dev jar), else the kritFir
        // configuration. Declared kritFir dependencies resolve through the project's
        // repositories, so offline builds, mirrors and dependency verification apply, and
        // fail the build when they cannot resolve. With none declared, kritFir defaults to
        // the krit-fir release matching the krit version, resolved leniently: a project
        // without a repository that has it keeps krit's own download (GitHub, Maven
        // Central or KRIT_JAR_REPOSITORY). An explicit binary has no known version, and
        // KRIT_JAR_REPOSITORY names krit's own mirror, so neither gets the default.
        val jarRepositoryEnv = project.providers.environmentVariable("KRIT_JAR_REPOSITORY")
        val kritFir = project.configurations.create(KRIT_FIR_CONFIGURATION) {
            isCanBeConsumed = false
            isCanBeResolved = true
            isTransitive = false
            description = "The krit-fir jar krit runs FIR with."
        }
        val defaultFirVersion = toolVersionInput.map { KritBinaryResolver.normalizeVersion(it) }
            .zip(jarRepositoryEnv.orElse("")) { version, mirror -> if (mirror.isBlank()) version else "" }
        val defaultFir = project.configurations.create("kritFirDefault") {
            isCanBeConsumed = false
            isCanBeResolved = true
            isTransitive = false
            description = "The krit-fir release matching the krit version, when kritFir declares none."
            defaultDependencies {
                val version = defaultFirVersion.get()
                if (version.isNotEmpty()) add(project.dependencies.create("$KRIT_FIR_MODULE:$version"))
            }
        }
        val defaultFirFiles = defaultFir.incoming.artifactView { lenient(true) }.files
        val firJarEnv = project.providers.environmentVariable("KRIT_FIR_JAR")
        // Gated on each task's own fir property, which a task can set apart from the extension.
        val firJarInput = { taskFir: Provider<Boolean> ->
            project.files(taskFir.map { enabled ->
                when {
                    !enabled -> emptyList<Any>()
                    !firJarEnv.orNull.isNullOrBlank() -> listOf(File(firJarEnv.get()))
                    kritFir.dependencies.isNotEmpty() -> listOf(kritFir)
                    else -> listOf(defaultFirFiles)
                }
            })
        }

        // FIR JDK: Java toolchains, so FIR does not depend on the JDK the daemon has.
        project.pluginManager.apply(org.gradle.api.plugins.JvmToolchainsPlugin::class.java)
        val toolchains = project.extensions.getByType(JavaToolchainService::class.java)
        extension.firOptions.useBuiltDependencies.convention(false)
        extension.firOptions.javaLauncher.convention(toolchains.launcherFor {
            languageVersion.set(JavaLanguageVersion.of(maxOf(FIR_MIN_JAVA, JavaVersion.current().majorVersion.toInt())))
        })
        val firLauncher = { taskFir: Provider<Boolean> ->
            taskFir.flatMap { enabled ->
                if (enabled) extension.firOptions.javaLauncher else project.provider<JavaLauncher> { null }
            }
        }

        // Warm FIR JVMs shared by the build's krit tasks (and later builds of this root).
        extension.firOptions.maxDaemons.convention(2)
        val maxWorkers = project.gradle.startParameter.maxWorkerCount
        val poolSlots = extension.firOptions.maxDaemons.map { it.coerceIn(1, maxOf(1, maxWorkers)) }
        val poolName = "gradle-" + hashutilSha256(modelPath(project.rootDir)).take(12)
        val firPoolService = project.gradle.sharedServices.registerIfAbsent(
            KritFirDaemonPool.SERVICE_NAME,
            KritFirDaemonPool::class.java,
        ) {
            maxParallelUsages.set(poolSlots)
            parameters.poolName.set(poolName)
            parameters.slots.set(poolSlots)
        }
        // usesService must be declared while the task is configured, before a build script's
        // own `fir = ...` on the task applies, so the declaration (which caps how many krit
        // tasks run at once) follows the extension; Go-only builds stay unthrottled. The slot
        // lease follows the task's own fir. A task that turns FIR on against the extension
        // gets no pool and falls back to krit's own per-project daemon.
        val poolDeclared = { extension.fir.getOrElse(true) }
        val firPool = { taskFir: Provider<Boolean> ->
            taskFir.flatMap { enabled ->
                if (enabled && poolDeclared()) firPoolService else project.provider<KritFirDaemonPool> { null }
            }
        }
        val usePool: org.gradle.api.Task.() -> Unit = {
            if (poolDeclared()) usesService(firPoolService)
        }

        // Wire task defaults for all KritCheckTask instances
        project.tasks.withType(KritCheckTask::class.java).configureEach {
            kritBinary.convention(advanced.binary)
            toolVersion.convention(toolVersionInput)
            this.binaryResolver.convention(binaryResolver)
            usesService(binaryResolver)
            firJar.from(firJarInput(fir))
            javaLauncher.convention(firLauncher(fir))
            firDaemonPool.convention(firPool(fir))
            firMaxHeap.convention(extension.firOptions.maxHeap)
            usePool()
            allRules.convention(advanced.allRules)
            ignoreFailures.convention(extension.ignoreFailures)
            config.convention(extension.config)
            baseline.convention(extension.baseline)
            parallel.convention(advanced.parallel)
            noCache.convention(advanced.noCache)
            typeInference.convention(advanced.typeInference)
            fir.convention(extension.fir)
            customRuleJars.from(extension.customRuleJars)
            // Wire reports from extension
            sarifRequired.convention(extension.reports.sarif.required)
            sarifOutput.convention(extension.reports.sarif.outputLocation)
            jsonRequired.convention(extension.reports.json.required)
            jsonOutput.convention(extension.reports.json.outputLocation)
            plainRequired.convention(extension.reports.plain.required)
            plainOutput.convention(extension.reports.plain.outputLocation)
            checkstyleRequired.convention(extension.reports.checkstyle.required)
            checkstyleOutput.convention(extension.reports.checkstyle.outputLocation)
            basePath.convention(project.layout.projectDirectory)
        }

        // Wire task defaults for the root kritCheckAll (dev.jasonpearson.krit.aggregate)
        project.tasks.withType(KritCheckAllTask::class.java).configureEach {
            kritBinary.convention(advanced.binary)
            toolVersion.convention(toolVersionInput)
            this.binaryResolver.convention(binaryResolver)
            usesService(binaryResolver)
            firJar.from(firJarInput(fir))
            javaLauncher.convention(firLauncher(fir))
            firDaemonPool.convention(firPool(fir))
            firMaxHeap.convention(extension.firOptions.maxHeap)
            usePool()
            allRules.convention(advanced.allRules)
            ignoreFailures.convention(extension.ignoreFailures)
            config.convention(extension.config)
            baseline.convention(extension.baseline)
            parallel.convention(advanced.parallel)
            noCache.convention(advanced.noCache)
            typeInference.convention(advanced.typeInference)
            fir.convention(extension.fir)
            moduleShards.convention(advanced.moduleShards)
            customRuleJars.from(extension.customRuleJars)
            sarifRequired.convention(extension.reports.sarif.required)
            jsonRequired.convention(extension.reports.json.required)
            plainRequired.convention(extension.reports.plain.required)
            checkstyleRequired.convention(extension.reports.checkstyle.required)
        }

        // Wire task defaults for all KritFormatTask instances
        project.tasks.withType(KritFormatTask::class.java).configureEach {
            kritBinary.convention(advanced.binary)
            toolVersion.convention(toolVersionInput)
            this.binaryResolver.convention(binaryResolver)
            usesService(binaryResolver)
            // kritFormat has no fir switch of its own; it follows the extension.
            firJar.from(firJarInput(extension.fir))
            javaLauncher.convention(firLauncher(extension.fir))
            firDaemonPool.convention(firPool(extension.fir))
            firMaxHeap.convention(extension.firOptions.maxHeap)
            usePool()
            config.convention(extension.config)
            fixLevel.convention(advanced.fixLevel)
            parallel.convention(advanced.parallel)
            noCache.convention(advanced.noCache)
            typeInference.convention(advanced.typeInference)
        }

        // Wire task defaults for all KritBaselineTask instances
        project.tasks.withType(KritBaselineTask::class.java).configureEach {
            kritBinary.convention(advanced.binary)
            toolVersion.convention(toolVersionInput)
            this.binaryResolver.convention(binaryResolver)
            usesService(binaryResolver)
            firJar.from(firJarInput(fir))
            javaLauncher.convention(firLauncher(fir))
            firDaemonPool.convention(firPool(fir))
            firMaxHeap.convention(extension.firOptions.maxHeap)
            usePool()
            config.convention(extension.config)
            allRules.convention(advanced.allRules)
            parallel.convention(advanced.parallel)
            noCache.convention(advanced.noCache)
            typeInference.convention(advanced.typeInference)
            customRuleJars.from(extension.customRuleJars)
            basePath.convention(project.layout.projectDirectory)
            fir.convention(extension.fir)
        }

        // Register the aggregate kritCheck task
        project.tasks.register("kritCheck", KritCheckTask::class.java) {
            setSource(advanced.source)
            sourceRoots.from(advanced.source)
            description = "Run krit analysis on all Kotlin sources"
        }

        registerModelExport(project, extension)

        // Register the kritFormat task
        project.tasks.register("kritFormat", KritFormatTask::class.java) {
            source.setFrom(advanced.source)
            description = "Apply krit auto-fixes to Kotlin sources"
        }

        // Register the kritBaseline task
        project.tasks.register("kritBaseline", KritBaselineTask::class.java) {
            source.setFrom(advanced.source)
            baselineFile.convention(
                project.layout.buildDirectory.file("reports/krit/baseline.xml")
            )
            description = "Create a krit baseline file from current findings"
        }

        // Wire kritCheck into the check lifecycle if available
        project.plugins.withType(org.gradle.language.base.plugins.LifecycleBasePlugin::class.java) {
            project.tasks.named("check") { dependsOn("kritCheck") }
        }

        // Register per-source-set tasks for Kotlin JVM projects
        registerKotlinJvmSourceSetTasks(project, extension)

    }

    private fun registerModelExport(project: Project, extension: KritExtension) {
        var modelExported = false
        var warnIfAndroidAppliedLate = false
        val variantSources = AndroidVariantSources(project)
        val exportModel = org.gradle.api.Action<Project> {
            if (modelExported) return@Action
            val kmp = project.plugins.hasPlugin(KOTLIN_MULTIPLATFORM_PLUGIN_ID)
            val android = hasAndroidPlugin(project)
            if (!kmp && !android && !project.plugins.hasPlugin("java")) return@Action
            modelExported = true
            warnIfAndroidAppliedLate = !android
            var androidModelSkipped = false
            val specs = when {
                kmp -> {
                    val kmpSpecs = kmpModels(project, extension, android)
                    if (android && kmpSpecs.none { it.platform == "android" }) {
                        val androidModels = androidModels(project, extension)
                        androidModelSkipped = androidModels.skipped
                        kmpSpecs + androidModels.specs
                    } else kmpSpecs
                }
                android -> {
                    val androidModels = androidModels(project, extension)
                    androidModelSkipped = androidModels.skipped
                    androidModels.specs
                }
                else -> jvmModels(project)
            }
            if (android && specs.none { it.platform == "android" } && !androidModelSkipped) {
                project.logger.warn("krit: kritExportModel for ${project.path} is exporting zero Android source sets; " +
                    "check Android variant configuration and krit.androidVariant (requested '${extension.androidVariant.orNull ?: "debug"}')")
            }
            // Test source sets are left out before any of their classpaths is wired, so none resolves.
            val exported = if (extension.exportTests.get()) specs else specs.filter { it.kind == "main" }
            registerProjectModel(project, extension, exported, variantSources)
        }
        fun exportAfterEvaluateOrNow() {
            if (modelExported) {
                if (warnIfAndroidAppliedLate) {
                    warnIfAndroidAppliedLate = false
                    project.logger.warn("krit: kritExportModel for ${project.path} was configured before Android was applied; " +
                        "the model does not include Android source sets. Apply Android in the plugins block.")
                }
                return
            }
            if (project.state.executed) {
                exportModel.execute(project)
            } else {
                project.afterEvaluate(exportModel)
            }
        }
        // Application, library, test (instrumentation, benchmark and baseline-profile
        // modules) and dynamic-feature modules share AGP's CommonExtension shape and
        // export through the same Android path.
        ANDROID_PLUGIN_IDS.forEach { id ->
            project.pluginManager.withPlugin(id) {
                // onVariants callbacks must exist before AGP creates its variants.
                variantSources.register()
                exportAfterEvaluateOrNow()
            }
        }
        // The Android-KMP plugin has the Variant API too, but no top-level `android`
        // extension: its target exports through the KMP path, after evaluation.
        project.pluginManager.withPlugin(ANDROID_KMP_LIBRARY_PLUGIN_ID) { variantSources.register() }
        // Android applied from an afterEvaluate is not supported: Android-only projects get the skipped Android model warning;
        // JVM and KMP projects keep their model and get the 'configured before Android' warning.
        project.afterEvaluate {
            if (!modelExported && !hasAndroidPlugin(project)) {
                exportModel.execute(project)
            }
        }
    }

    private fun hasAndroidPlugin(project: Project): Boolean =
        ANDROID_PLUGIN_IDS.any { project.plugins.hasPlugin(it) }

    /**
     * The project a `com.android.test` module tests (`android.targetProjectPath`). The
     * module compiles against that app's classes, which AGP provides through the tested
     * variant rather than an ordinary project dependency, so it is recorded as one.
     */
    private fun androidTargetProjectPath(project: Project, android: Any): String? {
        if (!project.plugins.hasPlugin(ANDROID_TEST_PLUGIN_ID)) return null
        val path = runCatching { android.javaClass.getMethod("getTargetProjectPath").invoke(android) }
            .getOrNull()?.toString()?.takeIf { it.startsWith(":") }
        if (path == null) project.logger.warn("krit: com.android.test module ${project.path} has no android.targetProjectPath; " +
            "its model has no dependency on the tested app")
        return path
    }

    private data class KotlinCompilerOptionsModel(
        val languageVersion: String = "",
        val apiVersion: String = "",
        val optIn: List<String> = emptyList(),
        val freeCompilerArgs: List<String> = emptyList(),
        val progressiveMode: Boolean = false,
        val pluginVersion: String = "",
    ) : java.io.Serializable

    private data class ModelSpec(
        val name: String, val kind: String, val platform: String, val variant: String,
        val dirs: List<String>, val boot: Provider<List<String>>, val configuration: Configuration,
        val compileTask: String,
        /** Project paths the compilation depends on beyond [configuration]'s project dependencies. */
        val extraProjectDeps: List<String> = emptyList(),
        /** Source roots read when the export runs (generated roots plugins register late), merged into [dirs]. */
        val lateDirs: Provider<List<String>>? = null,
        /** The compilation's source roots with their producer tasks; set for production compilations only. */
        val producers: FileCollection? = null,
        /** True when [producers] covers the Kotlin roots, so KSP needs no task-name fallback. */
        val kotlinRootsKnown: Boolean = false,
        /**
         * The AGP component (`debug`, `debugUnitTest`, ...) an Android compilation belongs to, when
         * it differs from [name]: a KMP Android target's `androidDebug` is AGP's `debug`.
         */
        val component: String = "",
        /** False when the compilation has no R class to predict a jar for (Android-KMP with resources off). */
        val rJarExpected: Boolean = true,
        /** True for a KMP test compilation, whose configuration holds its main compilation's outputs. */
        val ownOutputsOnClasspath: Boolean = false,
    )

    private data class AndroidModelsResult(val specs: List<ModelSpec>, val skipped: Boolean = false)

    private fun selectedAndroidVariant(project: Project, extension: KritExtension): String? {
        val requested = extension.androidVariant.orNull ?: "debug"
        val selected = pickAndroidVariant(project.configurations.map { it.name }
            .filter { it.endsWith("CompileClasspath") }
            .map { it.removeSuffix("CompileClasspath") }
            .filterNot { it.endsWith("UnitTest") || it.endsWith("AndroidTest") ||
                it.endsWith("TestFixtures") }, requested)
        if (selected != null) {
            val fallback = if (selected == requested) "" else "; falling back from requested '$requested'"
            project.logger.info("krit: kritExportModel for ${project.path} selected Android variant '$selected'$fallback")
        }
        return selected
    }

    private fun jvmModels(project: Project): List<ModelSpec> {
        val sourceSets = project.extensions.getByType(SourceSetContainer::class.java)
        val empty = project.provider { emptyList<String>() }
        val names = listOf("main", "test") +
            if (project.plugins.hasPlugin("java-test-fixtures")) listOf("testFixtures") else emptyList()
        return names.mapNotNull { name ->
            val config = project.configurations.findByName(
                if (name == "main") JavaPlugin.COMPILE_CLASSPATH_CONFIGURATION_NAME
                else "${name}CompileClasspath",
            ) ?: return@mapNotNull null
            val kotlinSets = KotlinGeneratedSources.jvmSourceSet(project, name)
                ?.let(KotlinGeneratedSources::directorySets).orEmpty()
            ModelSpec(name, name, "jvm", "",
                sourceSets.getByName(name).allSource.srcDirs.map(::modelPath), empty, config,
                if (name == "main") "compileKotlin" else "compile${name.replaceFirstChar(Char::uppercase)}Kotlin",
                lateDirs = kotlinSets.takeIf { it.isNotEmpty() }?.let { KotlinGeneratedSources.paths(project, it) },
                producers = if (name == "main") KotlinGeneratedSources.producers(project, kotlinSets, name) else null,
                kotlinRootsKnown = kotlinSets.isNotEmpty())
        }
    }

    private fun androidModels(project: Project, extension: KritExtension): AndroidModelsResult {
        val candidates = project.configurations.map { it.name }
            .filter { it.endsWith("CompileClasspath") }
            .map { it.removeSuffix("CompileClasspath") }
            .filterNot { it.endsWith("UnitTest") || it.endsWith("AndroidTest") ||
                it.endsWith("TestFixtures") }.sorted()
        val variant = selectedAndroidVariant(project, extension)
        val android = project.extensions.findByName("android")
        val components = project.extensions.findByName("androidComponents")
        if (variant == null || android == null || components == null) {
            project.logger.warn("krit: kritExportModel skipped Android model for ${project.path}: missing variant or Android extension; apply Android in the plugins block")
            return AndroidModelsResult(emptyList(), skipped = true)
        }
        return try {
            val boot = androidBootClasspath(project)
            val target = listOfNotNull(androidTargetProjectPath(project, android))
            // A com.android.test module has only `main` (no unit-test or androidTest
            // components), so the test and androidTest entries find no configuration.
            val entries = listOf(
                Triple(variant, "main", listOf("main", variant) + androidVariantFallbacks(variant)),
                Triple("${variant}UnitTest", "test", androidTestSourceNames("test", variant)),
                Triple("${variant}AndroidTest", "androidTest", androidTestSourceNames("androidTest", variant)),
            )
            val specs = entries.mapNotNull { (name, kind, sourceNames) ->
                val config = project.configurations.findByName("${name}CompileClasspath")
                    ?: return@mapNotNull null
                try {
                    ModelSpec(name, kind, "android", variant,
                        androidSourceDirs(project, android, sourceNames, kind == "main"),
                        boot, config, "compile${name.replaceFirstChar(Char::uppercase)}Kotlin",
                        if (kind == "main") target else emptyList())
                } catch (error: Exception) {
                    project.logger.warn("krit: Android model skipped source set $name in ${project.path}: ${error.message}")
                    null
                }
            }
            candidates.forEach { candidate ->
                val dirs = try {
                    androidSourceDirs(project, android,
                        listOf("main", candidate) + androidVariantFallbacks(candidate), true)
                } catch (error: Exception) {
                    project.logger.warn("krit: Android source set reflection failed for ${project.path} $candidate: ${error.message}")
                    emptyList()
                }
                if (dirs.isNotEmpty()) {
                    project.tasks.register("kritCheck${candidate.replaceFirstChar(Char::uppercase)}", KritCheckTask::class.java) {
                        dependsOn(extension.exportModel.map { enabled ->
                            if (enabled) listOf(project.tasks.named("kritExportModel")) else emptyList<Any>()
                        })
                        modelFile.from(project.provider {
                            if (extension.exportModel.get()) project.tasks.named("kritExportModel", KritExportModelTask::class.java).flatMap { it.outputFile }
                            else emptyList<Any>()
                        })
                        setSource(project.files(dirs))
                        sourceRoots.from(dirs)
                        description = "Run krit analysis on the '$candidate' variant sources"
                    }
                }
            }
            AndroidModelsResult(specs)
        } catch (error: Exception) {
            project.logger.warn("krit: kritExportModel skipped Android model for ${project.path}: ${error.message}")
            AndroidModelsResult(emptyList(), skipped = true)
        }
    }

    private fun androidVariantFallbacks(variant: String): List<String> =
        Regex("([A-Z][a-z0-9]*)$").find(variant)?.let { match ->
            listOfNotNull(match.value.replaceFirstChar(Char::lowercase),
                variant.substring(0, match.range.first).takeIf { it.isNotEmpty() }
                    ?.replaceFirstChar(Char::lowercase))
        } ?: emptyList()

    private fun androidTestSourceNames(prefix: String, variant: String): List<String> =
        listOf(prefix) + androidVariantFallbacks(variant).asReversed().map {
            prefix + it.replaceFirstChar(Char::uppercase)
        } + (prefix + variant.replaceFirstChar(Char::uppercase))

    private fun androidBootClasspath(project: Project): Provider<List<String>> {
        val components = project.extensions.findByName("androidComponents")
            ?: throw ReflectiveOperationException("androidComponents extension absent")
        val sdk = reflect(components, "getSdkComponents")
        @Suppress("UNCHECKED_CAST")
        val provider = reflect(sdk, "getBootClasspath") as Provider<List<RegularFile>>
        return provider.map { files -> bootClasspathPaths(files.map { it.asFile }) }
    }

    private fun androidSourceDirs(project: Project, android: Any, names: List<String>, requireMain: Boolean): List<String> {
        val sourceSets = android.javaClass.getMethod("getSourceSets").invoke(android)
        // AGP's source-set container supports findByName; retain getByName for
        // Android-shaped extensions that expose only the older lookup method.
        val findByName = try {
            sourceSets.javaClass.getMethod("findByName", String::class.java)
        } catch (_: NoSuchMethodException) {
            null
        }
        val getByName = if (findByName == null) {
            sourceSets.javaClass.getMethod("getByName", String::class.java)
        } else null
        return names.distinct().flatMap { name ->
            val sourceSet = if (findByName != null) {
                findByName.invoke(sourceSets, name)
            } else {
                try {
                    getByName!!.invoke(sourceSets, name)
                } catch (error: java.lang.reflect.InvocationTargetException) {
                    if ((name == "main" && requireMain) ||
                        (error.targetException !is org.gradle.api.UnknownDomainObjectException &&
                            error.targetException !is IllegalArgumentException &&
                            error.targetException !is NoSuchElementException)) throw error
                    null
                }
            }
            if (sourceSet == null) {
                // Only main is required. Variant, build-type, and flavor source
                // sets exist only when declared in the Android DSL.
                if (name == "main" && requireMain) throw ReflectiveOperationException("android.sourceSets has no main source set")
                project.logger.debug("krit: optional Android source set '$name' is absent for ${project.path}")
                return@flatMap emptyList()
            }
            listOf("Java", "Kotlin").flatMap { language ->
                val source = try {
                    sourceSet.javaClass.getMethod("get${language}").invoke(sourceSet)
                } catch (error: NoSuchMethodException) {
                    if (language == "Kotlin") return@flatMap emptyList()
                    throw error
                }
                @Suppress("UNCHECKED_CAST")
                (source.javaClass.getMethod("getSrcDirs").invoke(source) as Collection<File>)
                    .map(::modelPath)
            }
        }.distinct().sorted()
    }

    private fun kmpModels(project: Project, extension: KritExtension, android: Boolean): List<ModelSpec> {
        val kotlin = project.extensions.findByName("kotlin") ?: run {
            project.logger.warn("krit: KMP kotlin extension absent for ${project.path}")
            return emptyList()
        }
        val targets = try {
            reflect(kotlin, "getTargets") as Iterable<*>
        } catch (error: Exception) {
            project.logger.warn("krit: KMP targets reflection failed for ${project.path}: ${error.message}")
            return emptyList()
        }
        val specs = mutableListOf<ModelSpec>()
        // com.android.kotlin.multiplatform.library: one variant, so its Android target's
        // compilations are main / hostTest / deviceTest (the test ones opt-in) rather than
        // androidTarget()'s debug / debugUnitTest / debugAndroidTest.
        val androidKmp = project.plugins.hasPlugin(ANDROID_KMP_LIBRARY_PLUGIN_ID)
        targets.forEach { target ->
            if (target == null) return@forEach
            val targetName = runCatching { reflect(target, "getName").toString() }.getOrDefault("<unknown>")
            try {
                val targetSpecs = mutableListOf<ModelSpec>()
                val platformType = reflect(target, "getPlatformType")
                val platformName = runCatching { reflect(platformType, "getName").toString() }
                    .getOrElse { (platformType as? Enum<*>)?.name ?: platformType.toString() }
                val platform = when (platformName) {
                    "jvm" -> "jvm"
                    "androidJvm" -> "android"
                    else -> return@forEach
                }
                val compilations = reflect(target, "getCompilations")
                val androidKmpTarget = platform == "android" && androidKmp
                val variant = if (platform == "android" && !androidKmpTarget) {
                    selectedAndroidVariant(project, extension)
                } else null
                val requested = when {
                    androidKmpTarget -> androidKmpCompilations(project, targetName, compilations)
                    platform == "android" -> {
                        if (variant == null) {
                            project.logger.warn(
                                "krit: KMP Android variant absent for ${project.path} target $targetName")
                            return@forEach
                        }
                        listOf(variant to "main", "${variant}UnitTest" to "test",
                            "${variant}AndroidTest" to "androidTest")
                    }
                    else -> listOf("main" to "main", "test" to "test")
                }
                val boot = if (platform == "android" && (android || androidKmpTarget)) {
                    androidBootClasspath(project)
                } else project.provider { emptyList<String>() }
                for ((requestedName, kind) in requested) {
                    val compilation = findNamed(compilations, requestedName)
                    if (compilation == null) {
                        project.logger.warn("krit: KMP compilation missing in ${project.path} target $targetName: $requestedName")
                        continue
                    }
                    val configName = reflect(compilation, "getCompileDependencyConfigurationName").toString()
                    val configuration = project.configurations.findByName(configName)
                        ?: throw ReflectiveOperationException("missing configuration $configName")
                    val compilationName = runCatching { reflect(compilation, "getName").toString() }
                        .getOrDefault(requestedName)
                    val sourceSets = (reflect(compilation, "getKotlinSourceSets") as Iterable<*>).filterNotNull()
                    // The compilation also compiles its source sets' dependsOn closure (commonMain and
                    // intermediate sets, #1752): allKotlinSourceSets. KGP adds the default hierarchy's
                    // edges after this afterEvaluate, so the closure is read when the export runs.
                    val allSourceSets = {
                        ((runCatching { reflect(compilation, "getAllKotlinSourceSets") as Iterable<*> }.getOrNull()
                            ?: sourceSets).filterNotNull() + sourceSets).distinct()
                    }
                    val allKotlinSets = { allSourceSets().flatMap(KotlinGeneratedSources::directorySets) }
                    fun kotlinDirs(sourceSet: Any): List<String> {
                        val kotlinDirs = reflect(sourceSet, "getKotlin")
                        @Suppress("UNCHECKED_CAST")
                        return (reflect(kotlinDirs, "getSrcDirs") as Collection<File>).map(::modelPath)
                    }
                    val dirs = sourceSets.flatMap(::kotlinDirs).distinct().sorted()
                    val name = "${targetName}${compilationName.replaceFirstChar(Char::uppercase)}"
                    val compileTask = runCatching {
                        reflect(compilation, "getCompileKotlinTaskName").toString()
                    }.getOrDefault("compileKotlin${name.replaceFirstChar(Char::uppercase)}")
                    val kotlinSets = sourceSets.flatMap(KotlinGeneratedSources::directorySets)
                    val closureDirs = project.provider { allSourceSets().flatMap(::kotlinDirs) }
                    targetSpecs += ModelSpec(name, kind, platform, variant ?: name, dirs,
                        boot, configuration, compileTask,
                        lateDirs = closureDirs.zip(KotlinGeneratedSources.paths(project, allKotlinSets)) { a, b ->
                            (a + b).distinct().sorted()
                        },
                        producers = if (kind == "main") {
                            KotlinGeneratedSources.producers(project, allKotlinSets)
                        } else null,
                        kotlinRootsKnown = kotlinSets.isNotEmpty(),
                        // androidTarget()'s compilations are named after their AGP component; the
                        // Android-KMP target's single component is named like the spec (androidMain).
                        component = if (platform == "android" && !androidKmpTarget) requestedName else "",
                        rJarExpected = !androidKmpTarget || androidKmpResources(target),
                        ownOutputsOnClasspath = kind != "main")
                }
                specs += targetSpecs
            } catch (error: Exception) {
                project.logger.warn("krit: KMP model skipped target $targetName in ${project.path}: ${error.message}")
            }
        }
        return specs
    }

    /**
     * The Android-KMP target's compilations with their kinds. Its test compilations exist only
     * when enabled (`withHostTest`, `withDeviceTest`) and are told apart by AGP's types, not
     * names; a compilation of another type is left out.
     */
    private fun androidKmpCompilations(
        project: Project, targetName: String, compilations: Any,
    ): List<Pair<String, String>> {
        val found = (compilations as Iterable<*>).filterNotNull().mapNotNull { compilation ->
            val name = reflect(compilation, "getName").toString()
            val types = generateSequence<Class<*>>(compilation.javaClass) { it.superclass }
                .flatMap { sequenceOf(it) + it.interfaces.asSequence() }.map { it.name }.toSet()
            val kind = when {
                KMP_HOST_TEST_TYPE in types -> "test"
                KMP_DEVICE_TEST_TYPE in types -> "androidTest"
                name == "main" -> "main"
                else -> null
            }
            if (kind == null) {
                project.logger.info("krit: KMP model skipped compilation $name of ${project.path} " +
                    "target $targetName: not the Android-KMP main, host-test or device-test compilation")
            }
            kind?.let { name to it }
        }
        if (found.none { it.second == "main" }) {
            project.logger.warn("krit: KMP compilation missing in ${project.path} target $targetName: main")
        }
        return found
    }

    /** Whether the Android-KMP target processes resources (`androidResources { enable = true }`), so has an R class. */
    private fun androidKmpResources(target: Any): Boolean =
        runCatching { reflect(reflect(target, "getAndroidResources"), "getEnable") as Boolean }.getOrDefault(true)

    /**
     * The `kotlin` roots of every Kotlin source set of a KMP project, generated ones (under the
     * build directory) left out.
     */
    private fun kmpSourceDirs(project: Project): List<File> {
        val kotlin = project.extensions.findByName("kotlin") ?: return emptyList()
        val build = project.layout.buildDirectory.get().asFile.toPath().toAbsolutePath().normalize()
        return runCatching {
            (reflect(kotlin, "getSourceSets") as Iterable<*>).filterNotNull().flatMap { sourceSet ->
                @Suppress("UNCHECKED_CAST")
                reflect(reflect(sourceSet, "getKotlin"), "getSrcDirs") as Collection<File>
            }
        }.getOrElse { error ->
            project.logger.warn("krit: KMP source sets unavailable for ${project.path}: ${error.message}")
            emptyList()
        }.filterNot { it.toPath().toAbsolutePath().normalize().startsWith(build) }.distinct().sorted()
    }

    /**
     * True for a file dependency's artifact (Gradle's opaque identifier, which carries its file)
     * under [ownBuild]. Other identifiers carry no file and are kept.
     */
    private fun isOwnBuildOutput(id: ComponentIdentifier, ownBuild: java.nio.file.Path): Boolean {
        val file = runCatching { id.javaClass.getMethod("getFile").invoke(id) as? File }.getOrNull() ?: return false
        return file.toPath().toAbsolutePath().normalize().startsWith(ownBuild)
    }

    private fun reflect(value: Any, method: String): Any =
        value.javaClass.getMethod(method).invoke(value)
            ?: throw ReflectiveOperationException("$method returned null")

    private fun findNamed(container: Any, name: String): Any? {
        val find = container.javaClass.methods.firstOrNull {
            it.name == "findByName" && it.parameterTypes.contentEquals(arrayOf(String::class.java))
        }
        if (find != null) return find.invoke(container, name)
        return try {
            container.javaClass.getMethod("getByName", String::class.java).invoke(container, name)
        } catch (error: java.lang.reflect.InvocationTargetException) {
            if (error.targetException is org.gradle.api.UnknownDomainObjectException ||
                error.targetException is IllegalArgumentException ||
                error.targetException is NoSuchElementException) null else throw error
        }
    }

    private fun jvmTarget(project: Project, compileTask: String, name: String): Provider<String> {
        val kotlinTarget = if (project.tasks.names.contains(compileTask))
            project.tasks.named(compileTask).map { task ->
                try {
                    val target = reflect(reflect(task, "getCompilerOptions"), "getJvmTarget")
                    val value = if (target is Provider<*>) target.orNull else target
                    value?.let { resolved -> runCatching { reflect(resolved, "getTarget").toString() }
                        .getOrElse { resolved.toString() } } ?: ""
                } catch (_: Exception) { "" }
            } else project.provider { "" }
        val javaName = if (name == "main") "compileJava"
            else "compile${name.replaceFirstChar(Char::uppercase)}Java"
        val javaTarget = if (project.tasks.names.contains(javaName))
            project.tasks.named(javaName).map { (it as? JavaCompile)?.targetCompatibility ?: "" }
            else project.provider { "" }
        val java = project.extensions.findByName("java")
        val toolchainTarget = try {
            val version = reflect(reflect(java ?: throw ReflectiveOperationException(), "getToolchain"),
                "getLanguageVersion")
            if (version is Provider<*>) version.map { it.toString() } else project.provider { version.toString() }
        } catch (_: Exception) { project.provider { "" } }
        return project.provider {
            kotlinTarget.orNull?.takeIf(String::isNotBlank)
                ?: javaTarget.orNull?.takeIf(String::isNotBlank)
                ?: toolchainTarget.orNull.orEmpty()
        }
    }

    /**
     * The Kotlin compiler settings of one compilation, read lazily from the
     * compile task's `compilerOptions` (KGP 1.8+) or, for older plugins, the
     * deprecated `kotlinOptions`. Every read is guarded on its own so a KGP
     * that lacks one option (or a build without KGP, where the task is absent)
     * still exports the rest; an unreadable option is exported as unset.
     */
    private fun kotlinCompilerOptions(project: Project, compileTask: String): Provider<KotlinCompilerOptionsModel> {
        val pluginVersion = kotlinPluginVersion(project)
        if (!project.tasks.names.contains(compileTask)) {
            return project.provider { KotlinCompilerOptionsModel(pluginVersion = pluginVersion) }
        }
        // Wrapped in provider{} rather than TaskProvider.map{}: mapping a task
        // provider would make the export task depend on running the compile task.
        val task = project.tasks.named(compileTask)
        return project.provider { readKotlinCompilerOptions(task.get(), pluginVersion) }
    }

    /**
     * The Kotlin Gradle plugin's own version, which fixes the language version Gradle
     * compiled with when none is configured. Read from KGP's `getKotlinPluginVersion`
     * (resolved through the kotlin extension's class loader, since KGP is not on this
     * plugin's classpath). `coreLibrariesVersion` is only a last resort: it is user
     * overridable and often pinned below the plugin version.
     */
    private fun kotlinPluginVersion(project: Project): String {
        val kotlin = project.extensions.findByName("kotlin") ?: return ""
        try {
            val wrapper = kotlin.javaClass.classLoader
                .loadClass("org.jetbrains.kotlin.gradle.plugin.KotlinPluginWrapperKt")
            val version = wrapper.getMethod("getKotlinPluginVersion", Project::class.java).invoke(null, project)
            if (version != null && version.toString().isNotBlank()) return version.toString()
        } catch (_: Exception) {
        }
        return try { reflect(kotlin, "getCoreLibrariesVersion").toString() } catch (_: Exception) { "" }
    }

    private fun readKotlinCompilerOptions(task: Any, pluginVersion: String): KotlinCompilerOptionsModel {
        val options = runCatching { reflect(task, "getCompilerOptions") }.getOrNull()
        val legacy = if (options == null) runCatching { reflect(task, "getKotlinOptions") }.getOrNull() else null
        fun value(getter: String): Any? {
            val holder = options ?: legacy ?: return null
            val raw = runCatching { reflect(holder, getter) }.getOrNull() ?: return null
            return if (raw is Provider<*>) raw.orNull else raw
        }
        fun text(getter: String): String {
            val raw = value(getter) ?: return ""
            // KotlinVersion exposes the "2.0" spelling through getVersion().
            return runCatching { reflect(raw, "getVersion").toString() }.getOrElse { raw.toString() }
        }
        fun list(getter: String): List<String> =
            (value(getter) as? Iterable<*>)?.mapNotNull { it?.toString() }.orEmpty()
        return KotlinCompilerOptionsModel(
            languageVersion = text("getLanguageVersion"),
            apiVersion = text("getApiVersion"),
            optIn = list("getOptIn"),
            freeCompilerArgs = list("getFreeCompilerArgs"),
            progressiveMode = value("getProgressiveMode") == true,
            pluginVersion = pluginVersion,
        )
    }

    /**
     * The compile task's compiler plugin classpath (KGP's `pluginClasspath`: the applied compiler plugins, plus
     * KGP's own scripting support, which krit filters out), read lazily like the compiler options. Empty without
     * a Kotlin compile task or on a KGP that lacks the property.
     */
    private fun compilerPluginClasspath(project: Project, compileTask: String): Provider<FileCollection> {
        if (!project.tasks.names.contains(compileTask)) return project.provider { project.files() }
        val task = project.tasks.named(compileTask)
        val none = project.files()
        return project.provider {
            runCatching { reflect(task.get(), "getPluginClasspath") as? FileCollection }.getOrNull() ?: none
        }
    }

    /**
     * The compile task's compiler plugin options as `plugin:<id>:<key>=<value>`, from KGP's `pluginOptions`
     * (`CompilerPluginConfig.allOptions()`, KGP 1.7+), in the order KGP passes them. An option that cannot be read
     * is left out; a KGP without the property exports none.
     */
    private fun compilerPluginOptions(project: Project, compileTask: String): Provider<List<String>> {
        if (!project.tasks.names.contains(compileTask)) return project.provider { emptyList() }
        val task = project.tasks.named(compileTask)
        return project.provider { runCatching { readCompilerPluginOptions(task.get()) }.getOrElse { emptyList() } }
    }

    private fun readCompilerPluginOptions(task: Any): List<String> {
        val raw = reflect(task, "getPluginOptions")
        val configs = ((if (raw is Provider<*>) raw.orNull else raw) as? Iterable<*>)?.filterNotNull().orEmpty()
        val out = mutableListOf<String>()
        configs.forEach { config ->
            val all = runCatching { config.javaClass.getMethod("allOptions").invoke(config) }.getOrNull() as? Map<*, *>
            all?.entries?.forEach { entry ->
                (entry.value as? Iterable<*>)?.filterNotNull()?.forEach { option ->
                    val key = runCatching { reflect(option, "getKey").toString() }.getOrNull()
                    val value = runCatching { reflect(option, "getValue").toString() }.getOrNull()
                    if (key != null && value != null) out += "plugin:${entry.key}:$key=$value"
                }
            }
        }
        return out
    }

    /**
     * Where AGP writes the module's own R jar: `intermediates/<folder>/<component>/<task>/R.jar`,
     * from AGP's major version and task names. Only the path is computed. Listing the build
     * directory here would make it a configuration-cache input, so the first build that runs
     * the R task would discard the entry (#1645). krit looks the jar up when it runs, also under
     * the other folder names AGP uses for it (`gradlemodel.resolveRClassJars`).
     */
    private fun androidRJar(project: Project, variant: String, kind: String, major: Int): Provider<List<String>> {
        val libraryMain = kind == "main" && project.plugins.hasPlugin(ANDROID_LIBRARY_PLUGIN_ID)
        val predictedFolder = if (libraryMain) "compile_r_class_jar"
            else if (major == 8) "compile_and_runtime_not_namespaced_r_class_jar"
            else "compile_and_runtime_r_class_jar"
        val directory = androidRComponent(variant, kind)
        val task = androidRTask(variant, kind, libraryMain)
        return project.layout.buildDirectory.map { build ->
            listOf(modelPath(build.asFile.resolve("intermediates").resolve(predictedFolder)
                .resolve(directory).resolve(task).resolve("R.jar")))
        }
    }

    /** The AGP component whose R jar a spec of [kind] compiles against (`debug`, `debugUnitTest`, ...). */
    private fun androidRComponent(variant: String, kind: String): String = when (kind) {
        "test" -> "${variant}UnitTest"
        "androidTest" -> "${variant}AndroidTest"
        else -> variant
    }

    /** The AGP task that writes that component's R jar. */
    private fun androidRTask(variant: String, kind: String, libraryMain: Boolean): String {
        val capitalized = androidRComponent(variant, kind).replaceFirstChar(Char::uppercase)
        return when (kind) {
            "test" -> "generate${capitalized}StubRFile"
            "androidTest" -> "process${capitalized}Resources"
            else -> if (libraryMain) "generate${capitalized}RFile" else "process${capitalized}Resources"
        }
    }

    /**
     * The R tasks of the exported Android specs' components. A production component lists both
     * names, since which one AGP registers depends on the module type; a test component lists its
     * own, whose R jar the model records for it (#1724).
     */
    private fun androidRTasks(specs: List<ModelSpec>): List<String> =
        specs.filter { it.platform == "android" && it.variant.isNotEmpty() }
            .map { it.variant to it.kind }.distinct().flatMap { (variant, kind) ->
                val libraryMain = if (kind == "test" || kind == "androidTest") listOf(false) else listOf(true, false)
                libraryMain.map { androidRTask(variant, kind, it) }
            }

    private fun registerProjectModel(
        project: Project, extension: KritExtension, specs: List<ModelSpec>, variantSources: AndroidVariantSources,
    ) {
        val path = project.path
        val fileName = if (path == ":") "_root" else path.removePrefix(":").replace(":", "__")
        val rootPath = modelPath(project.rootDir)
        val androidSpecs = specs.filter { it.platform == "android" }
        val components = project.extensions.findByName("androidComponents")
        val major = if (androidSpecs.isNotEmpty()) runCatching {
            (reflect(reflect(components!!, "getPluginVersion"), "getMajor") as Number).toInt()
        }.getOrNull() else null
        val resolvedMajor = if (project.extensions.findByName("android") != null)
            major?.takeIf { it in 8..9 } else null
        // The Android-KMP plugin has no `android` extension, so no R jar is predicted for it. Its
        // resources are opt-in: with them off there is no R class and nothing to warn about.
        if (androidSpecs.any { it.rJarExpected } && resolvedMajor == null)
            project.logger.warn("krit: own Android R jar location unavailable for ${project.path}")
        val rJars = androidSpecs
            .map { it.variant to it.kind }.distinct()
            .associateWith { (variant, kind) ->
                if (resolvedMajor != null) androidRJar(project, variant, kind, resolvedMajor)
                else project.provider { emptyList<String>() }
            }
        val entries = specs.map { spec ->
            project.objects.newInstance(KritModelSourceSet::class.java).apply {
                name.set(spec.name)
                kind.set(spec.kind)
                platform.set(spec.platform)
                variant.set(spec.variant)
                // Roots plugins register (AGP's Variant API, KGP's source sets), generated ones
                // included; the export task files those under the build directory as
                // generatedSourceDirs.
                val late = lateDirs(spec, variantSources)
                if (late != null) {
                    val declared = spec.dirs
                    sourceDirs.set(late.map { (declared + it).distinct().sorted() })
                } else {
                    sourceDirs.set(spec.dirs)
                }
                bootClasspath.set(spec.boot)
                jvmTarget.set(jvmTarget(project, spec.compileTask, spec.kind))
                kotlinPresent.set(project.tasks.names.contains(spec.compileTask))
                val kotlinOptions = kotlinCompilerOptions(project, spec.compileTask)
                kotlinLanguageVersion.set(kotlinOptions.map { it.languageVersion })
                kotlinApiVersion.set(kotlinOptions.map { it.apiVersion })
                kotlinOptIn.set(kotlinOptions.map { it.optIn })
                kotlinFreeCompilerArgs.set(kotlinOptions.map { it.freeCompilerArgs })
                kotlinProgressiveMode.set(kotlinOptions.map { it.progressiveMode })
                kotlinPluginVersion.set(kotlinOptions.map { it.pluginVersion })
                kotlinCompilerPluginClasspath.from(compilerPluginClasspath(project, spec.compileTask))
                kotlinCompilerPluginOptions.set(compilerPluginOptions(project, spec.compileTask))
                // Only a KMP test compilation drops file dependencies under this project's build
                // directory: KGP adds its main compilation's outputs there (#1752), which the
                // model's source context covers. Elsewhere such a file is a real dependency (a jar
                // a task copied to build/), and stays.
                val ownBuild = if (spec.ownOutputsOnClasspath) {
                    project.layout.buildDirectory.get().asFile.toPath().toAbsolutePath().normalize()
                } else null
                val externalArtifacts = spec.configuration.incoming.artifactView {
                    if (spec.platform == "android") {
                        attributes.attribute(ArtifactTypeDefinition.ARTIFACT_TYPE_ATTRIBUTE, "android-classes-jar")
                    }
                    componentFilter { id ->
                        id !is ProjectComponentIdentifier && (ownBuild == null || !isOwnBuildOutput(id, ownBuild))
                    }
                }.files
                // The view still carries a filtered-out file dependency's producer (compileKotlinJvm), so
                // a KMP test compilation's classpath is read through a provider, which carries none:
                // exporting must not compile main code.
                classpath.from(
                    if (spec.ownOutputsOnClasspath) project.provider { externalArtifacts.files } else externalArtifacts,
                )
                val extraProjectDeps = spec.extraProjectDeps
                projectDeps.set(spec.configuration.incoming.resolutionResult.rootComponent.map { root ->
                    (root.dependencies.filterIsInstance<ResolvedDependencyResult>()
                        .mapNotNull { (it.selected.id as? ProjectComponentIdentifier)?.projectPath } + extraProjectDeps)
                        .distinct().sorted()
                })
                generatedClasspath.set(rJars[spec.variant to spec.kind] ?: project.provider { emptyList() })
            }
        }
        val export = project.tasks.register("kritExportModel", KritExportModelTask::class.java) {
            description = "Export this project's resolved compile classpath for krit"
            sourceSets.set(entries)
            buildDir.set(modelPath(project.layout.buildDirectory.get().asFile))
            projectPath.set(path)
            projectDir.set(modelPath(project.projectDir))
            generatedBy.set("krit-gradle-plugin ${KritVersion.VERSION}")
            rootDir.set(rootPath)
            gradleUserHome.set(modelPath(project.gradle.gradleUserHomeDir))
            outputFile.set(File(rootPath, ".krit/gradle-model/${fileName}.json"))
        }
        val generators = generatorDependencies(project, specs, variantSources)
        if (extension.exportGenerated.get()) export.configure { dependsOn(generators) }
        // Published for the root kritCheckAll (KritAggregatePlugin), which resolves every
        // project's model through a project dependency. The primary artifact is built by the
        // export alone. A secondary variant, `krit.generated=true`, is the same file built by
        // the generators too: kritCheckAll selects it when its own FIR is on, so it runs them
        // before FIR compiles the roots the model lists (#1686), and a Go-only aggregate scan
        // runs none. The choice is the consumer's, and nothing crosses projects.
        project.configurations.create(KRIT_MODEL_ELEMENTS) {
            isCanBeConsumed = true
            isCanBeResolved = false
            attributes.attribute(Category.CATEGORY_ATTRIBUTE,
                project.objects.named(Category::class.java, KRIT_GRADLE_MODEL_CATEGORY))
            attributes.attribute(KRIT_GENERATED_ATTRIBUTE, false)
            outgoing.artifact(export.flatMap { it.outputFile })
            // An artifact's builtBy resolves no task names, so names become task providers.
            val generatorTasks = generators.map { if (it is String) project.tasks.named(it) else it }
            outgoing.variants.create("generated") {
                attributes.attribute(KRIT_GENERATED_ATTRIBUTE, true)
                artifact(export.flatMap { it.outputFile }) { builtBy(generatorTasks) }
            }
        }
        // FIR compiles the model's generated roots, so they must exist when it runs: with FIR on,
        // every check task of this project (kritCheck, kritCheck<Variant>, kritCheck<SourceSet>,
        // and the root's kritCheckAll) runs the generators, never a compile, whether or not
        // exportGenerated is set. The list covers every exported production compilation, which
        // is what FIR compiles as context for any of these tasks' sources (#1642).
        project.tasks.withType(KritCheckTask::class.java).configureEach {
            dependsOn(fir.map { enabled -> if (enabled) generators else emptyList() })
        }
        project.tasks.withType(KritCheckAllTask::class.java).configureEach {
            dependsOn(fir.map { enabled -> if (enabled) generators else emptyList() })
        }
        // kritBaseline records the findings kritCheck reports, so it sees the same model and
        // generated roots.
        project.tasks.withType(KritBaselineTask::class.java).configureEach {
            dependsOn(extension.exportModel.map { enabled -> if (enabled) listOf(export) else emptyList<Any>() })
            dependsOn(fir.map { enabled -> if (enabled) generators else emptyList() })
        }
        project.tasks.named("kritCheck") {
            dependsOn(extension.exportModel.map { enabled ->
                if (enabled) listOf(export) else emptyList<Any>()
            })
            // A whole-directory input would read other projects' outputs and break
            // isolated configuration. Cache invalidation is project-local here;
            // export all models first when relying on cross-project model changes.
            (this as KritCheckTask).modelFile.from(project.provider {
                if (extension.exportModel.get()) export.flatMap { it.outputFile }
                else emptyList<Any>()
            })
        }
        wireBuiltDependencies(project, extension, specs)
    }

    /**
     * `krit { fir { useBuiltDependencies = true } }`: every check task of this project
     * gets the compiled outputs of its project dependencies, read from the exported
     * source sets' compile classpaths (jars for JVM, `android-classes-jar`
     * for Android). Gradle wires the upstream compile tasks, so their results come from
     * the build cache when available, and krit no longer compiles those projects'
     * sources as context (`--gradle-project-outputs`).
     */
    private fun wireBuiltDependencies(project: Project, extension: KritExtension, specs: List<ModelSpec>) {
        val enabled = extension.firOptions.useBuiltDependencies
        // A dependency's identity is its build-tree path: an included build's `:lib` is not
        // this build's `:lib` (Gradle 8.2+; older Gradle falls back to the project path).
        val buildPrefix = runCatching { project.buildTreePath.removeSuffix(project.path) }.getOrDefault("")
        val self = project.path
        fun sameBuild(id: ProjectComponentIdentifier): Boolean = runCatching {
            id.buildTreePath == if (id.projectPath == ":") buildPrefix.ifEmpty { ":" } else buildPrefix + id.projectPath
        }.getOrDefault(true)
        val views = specs.map { spec ->
            spec.configuration.incoming.artifactView {
                if (spec.platform == "android") {
                    attributes.attribute(ArtifactTypeDefinition.ARTIFACT_TYPE_ATTRIBUTE, "android-classes-jar")
                } else {
                    // Jars, not class directories: krit's caches key a classpath entry by its
                    // size and modification time, which a directory keeps when the classes in it
                    // are recompiled in place.
                    attributes.attribute(LibraryElements.LIBRARY_ELEMENTS_ATTRIBUTE,
                        project.objects.named(LibraryElements::class.java, LibraryElements.JAR))
                }
                componentFilter { id -> id is ProjectComponentIdentifier && sameBuild(id) && id.projectPath != self }
            }
        }
        // Resolved artifacts are read lazily (when the task runs), after the upstream jars.
        var outputs: Provider<Map<String, List<String>>> = project.provider { emptyMap() }
        views.forEach { view ->
            outputs = outputs.zip(view.artifacts.resolvedArtifacts) { acc, artifacts ->
                val merged = acc.mapValues { it.value.toMutableList() }.toMutableMap()
                artifacts.forEach { artifact ->
                    val id = artifact.id.componentIdentifier as? ProjectComponentIdentifier ?: return@forEach
                    val files = merged.getOrPut(id.projectPath) { mutableListOf() }
                    val path = modelPath(artifact.file)
                    if (path !in files) files += path
                }
                merged
            }
        }
        val upstream = views.map { it.files }
        project.tasks.withType(KritCheckTask::class.java).configureEach {
            useBuiltDependencies.convention(enabled)
            // Each task's own switches decide, so a task that turns FIR or built dependencies
            // off neither builds the upstream jars nor passes them.
            val on = useBuiltDependencies.zip(fir) { built, firOn -> built && firOn }
            projectOutputs.set(on.flatMap { if (it) outputs else project.provider { emptyMap() } })
            dependencyClasses.from(project.files(java.util.concurrent.Callable {
                if (on.getOrElse(false)) upstream else emptyList()
            }))
        }
    }

    /** The AGP component whose Variant API roots an Android spec has, or null. */
    private fun agpComponent(spec: ModelSpec, variantSources: AndroidVariantSources): String? =
        agpComponentName(spec)?.takeIf(variantSources::has)

    /**
     * The AGP component of an Android spec, whether or not AGP has created it yet: the
     * Android-KMP plugin creates its variant after the export is configured, so its roots are
     * read lazily through this name.
     */
    private fun agpComponentName(spec: ModelSpec): String? =
        spec.component.ifEmpty { spec.name }.takeIf { spec.platform == "android" }

    /**
     * The spec's late source roots: KGP's source sets, AGP's Variant API roots of its component, or
     * both for a KMP Android compilation, whose generated roots (view binding, BuildConfig, AIDL,
     * `addGeneratedSourceDirectory`) only AGP knows (#1643).
     */
    private fun lateDirs(spec: ModelSpec, variantSources: AndroidVariantSources): Provider<List<String>>? {
        val agp = agpComponentName(spec)?.let(variantSources::paths)
        val kgp = spec.lateDirs
        return when {
            kgp == null -> agp
            agp == null -> kgp
            else -> kgp.zip(agp) { a, b -> (a + b).distinct().sorted() }
        }
    }

    private fun producers(project: Project, spec: ModelSpec, variantSources: AndroidVariantSources): FileCollection? {
        val agp = if (spec.kind == "main") agpComponentName(spec)?.let(variantSources::producers) else null
        val kgp = spec.producers
        return when {
            kgp == null -> agp
            agp == null -> kgp
            else -> project.files(kgp, agp)
        }
    }

    /**
     * What must run so the model's generated sources and R jars exist on disk, without compiling:
     *
     * - every production compilation's source roots, which carry their producer tasks (AGP's Variant
     *   API for Android; KGP's `kotlin`/`generatedKotlin` source sets for JVM and KMP). That covers
     *   view binding, BuildConfig, AIDL, SQLDelight, Wire, KSP and any generator that registers its
     *   output with the build, without task names. Test compilations' roots are recorded but not
     *   produced: their KSP/kapt runs against the compiled production classes.
     * - Android R-class tasks (`generate<Variant>RFile`, `process<Variant>Resources`, and for exported
     *   test components `generate<Variant>UnitTestStubRFile` and `process<Variant>AndroidTestResources`)
     *   by name: AGP has no public artifact for the R jar. They already depend on every registered
     *   resource producer (`variant.sources.res`), so generated resources need no wiring of their own.
     *   They link resources and write R classes; none compiles.
     * - kapt stub and kapt tasks, and KSP tasks for compilations with no roots provider, by name.
     */
    private fun generatorDependencies(
        project: Project, specs: List<ModelSpec>, variantSources: AndroidVariantSources,
    ): List<Any> {
        val main = specs.filter { it.kind == "main" }
        val androidNames = androidRTasks(specs)
        val providers = main.mapNotNull { producers(project, it, variantSources) }
        fun relevant(of: List<ModelSpec>) = of.map { it.name.lowercase() } +
            of.filter { it.platform == "android" }.map { it.variant.lowercase() }.filter { it.isNotEmpty() } +
            // A KMP compilation's target prefix (`jvm` of `jvmMain`), as in `kspKotlinJvm`.
            of.mapNotNull { Regex("^[a-z0-9]+(?=[A-Z])").find(it.name)?.value?.lowercase() } +
            of.filter { it.name == "main" }.map { "kotlin" }
        val kspRelevant = relevant(main.filterNot {
            it.kotlinRootsKnown || agpComponent(it, variantSources) != null
        })
        val kaptRelevant = relevant(main)
        val hasKsp = project.plugins.hasPlugin("com.google.devtools.ksp")
        val hasKapt = project.plugins.hasPlugin("org.jetbrains.kotlin.kapt") || project.plugins.hasPlugin("kotlin-kapt")
        // A test compilation's tasks contain its production one's name (kspDebugUnitTestKotlin), so
        // they are excluded by their `Test` segment (kspTestKotlin, kaptGenerateStubsTestKotlin,
        // kspDebugAndroidTestKotlin, kspTestKotlinJvm, ...).
        val testTask = Regex("Test(Fixtures)?(Kotlin|$)")
        val byName = project.tasks.names.filter { taskName ->
            val lower = taskName.lowercase()
            val generator = (lower.contains("kotlin") || lower.contains("generatestubs")) && !testTask.containsMatchIn(taskName)
            taskName in androidNames ||
                (generator && hasKsp && lower.startsWith("ksp") && kspRelevant.any { lower.contains(it) }) ||
                (generator && hasKapt && lower.startsWith("kapt") && kaptRelevant.any { lower.contains(it) })
        }
        return byName + providers
    }

    /**
     * When the Kotlin JVM plugin is applied, register kritCheck<SourceSet> tasks
     * for each Kotlin source set (e.g., kritCheckMain, kritCheckTest).
     */
    private fun registerKotlinJvmSourceSetTasks(project: Project, extension: KritExtension) {
        project.plugins.withId(KOTLIN_JVM_PLUGIN_ID) {
            project.afterEvaluate {
                val kotlinExtension = project.extensions.findByName("kotlin")
                if (kotlinExtension != null) {
                    // Use reflection to access source sets without a compile-time dependency
                    // on the Kotlin Gradle Plugin
                    val sourceSets = try {
                        val method = kotlinExtension.javaClass.getMethod("getSourceSets")
                        @Suppress("UNCHECKED_CAST")
                        method.invoke(kotlinExtension) as? Iterable<Any>
                    } catch (_: Exception) {
                        null
                    }

                    sourceSets?.forEach { sourceSet ->
                        val name = try {
                            sourceSet.javaClass.getMethod("getName").invoke(sourceSet) as String
                        } catch (_: Exception) {
                            return@forEach
                        }

                        val kotlinDirs = try {
                            val kotlinProp = sourceSet.javaClass.getMethod("getKotlin")
                            val kotlinSourceSet = kotlinProp.invoke(sourceSet)
                            val srcDirs = kotlinSourceSet.javaClass.getMethod("getSrcDirs")
                            @Suppress("UNCHECKED_CAST")
                            srcDirs.invoke(kotlinSourceSet) as? Set<File>
                        } catch (_: Exception) {
                            null
                        }

                        if (kotlinDirs != null) {
                            val taskName = "kritCheck${name.replaceFirstChar { it.uppercase() }}"
                            if (project.tasks.findByName(taskName) == null) {
                                project.tasks.register(taskName, KritCheckTask::class.java) {
                                    dependsOn(extension.exportModel.map { enabled ->
                                        if (enabled) listOf(project.tasks.named("kritExportModel")) else emptyList<Any>()
                                    })
                                    modelFile.from(project.provider {
                                        if (extension.exportModel.get()) project.tasks.named("kritExportModel", KritExportModelTask::class.java).flatMap { it.outputFile }
                                        else emptyList<Any>()
                                    })
                                    setSource(project.files(kotlinDirs))
                                    sourceRoots.from(kotlinDirs)
                                    description = "Run krit analysis on the '$name' source set"
                                }
                            }
                        }
                    }
                }
            }
        }
    }

    private fun hashutilSha256(text: String): String =
        java.security.MessageDigest.getInstance("SHA-256").digest(text.toByteArray())
            .joinToString("") { "%02x".format(it) }

    companion object {
        /** krit release downloaded when `advanced.toolVersion` is unset: the plugin's own version. */
        const val KRIT_DEFAULT_VERSION = KritVersion.VERSION

        /**
         * Category attribute value identifying a Krit custom-rule bundle
         * variant. Must stay in sync with the matching string in
         * `dev.jasonpearson.krit.custom`'s `KritCustomRulePlugin`.
         */
        const val KRIT_RULE_BUNDLE_CATEGORY = "krit-rule-bundle"

        /** Category of the consumable variant carrying a project's `kritExportModel` file. */
        const val KRIT_GRADLE_MODEL_CATEGORY = "krit-gradle-model"

        /** Consumable configuration that publishes the project's exported model. */
        const val KRIT_MODEL_ELEMENTS = "kritModelElements"

        /**
         * Marks the [KRIT_MODEL_ELEMENTS] artifact that is built by the project's generated-source
         * producers as well as its model export (`true`), against the export alone (`false`).
         */
        val KRIT_GENERATED_ATTRIBUTE: org.gradle.api.attributes.Attribute<Boolean> =
            org.gradle.api.attributes.Attribute.of("dev.jasonpearson.krit.generated", Boolean::class.javaObjectType)

        internal const val BINARY_RESOLVER_SERVICE = "kritBinaryResolver"

        /** Resolvable configuration holding the krit-fir jar (see [KritFirOptions]). */
        const val KRIT_FIR_CONFIGURATION = "kritFir"

        /** FIR needs Java 21 or newer. */
        internal const val FIR_MIN_JAVA = 21

        const val KOTLIN_JVM_PLUGIN_ID = "org.jetbrains.kotlin.jvm"
        const val KOTLIN_MULTIPLATFORM_PLUGIN_ID = "org.jetbrains.kotlin.multiplatform"
        const val ANDROID_APPLICATION_PLUGIN_ID = "com.android.application"
        const val ANDROID_LIBRARY_PLUGIN_ID = "com.android.library"
        const val ANDROID_TEST_PLUGIN_ID = "com.android.test"
        const val ANDROID_DYNAMIC_FEATURE_PLUGIN_ID = "com.android.dynamic-feature"

        /** kritCheck's source when the project is not Kotlin Multiplatform. */
        private val DEFAULT_SOURCE = listOf("src/main/kotlin", "src/test/kotlin")

        /** AGP's Kotlin Multiplatform library plugin: `kotlin { android { } }`, required for KMP from AGP 9. */
        const val ANDROID_KMP_LIBRARY_PLUGIN_ID = "com.android.kotlin.multiplatform.library"
        private const val ANDROID_KMP_DSL = "com.android.build.api.dsl"
        private const val KMP_HOST_TEST_TYPE = "$ANDROID_KMP_DSL.KotlinMultiplatformAndroidHostTestCompilation"
        private const val KMP_DEVICE_TEST_TYPE = "$ANDROID_KMP_DSL.KotlinMultiplatformAndroidDeviceTestCompilation"

        /** Android plugins whose modules export through the Android model path. */
        val ANDROID_PLUGIN_IDS = listOf(
            ANDROID_APPLICATION_PLUGIN_ID,
            ANDROID_LIBRARY_PLUGIN_ID,
            ANDROID_TEST_PLUGIN_ID,
            ANDROID_DYNAMIC_FEATURE_PLUGIN_ID,
        )

        /**
         * Plugin IDs that trigger krit's per-source-set / per-variant task
         * registration. The settings plugin's auto-application list mirrors
         * this so the two can never drift — adding an entry here should add
         * the same key to the settings plugin and the matching wiring above.
         */
        val LANGUAGE_PLUGIN_IDS = listOf(
            KOTLIN_JVM_PLUGIN_ID,
            KOTLIN_MULTIPLATFORM_PLUGIN_ID,
        ) + ANDROID_PLUGIN_IDS
    }
}
