package dev.jasonpearson.krit.gradle

import org.gradle.api.Plugin
import org.gradle.api.Project
import org.gradle.api.attributes.Category
import org.gradle.api.artifacts.component.ProjectComponentIdentifier
import org.gradle.api.artifacts.result.ResolvedDependencyResult
import org.gradle.api.artifacts.type.ArtifactTypeDefinition
import org.gradle.api.plugins.JavaPlugin
import org.gradle.api.tasks.SourceSetContainer
import org.gradle.api.provider.Provider
import org.gradle.api.file.RegularFile
import org.gradle.api.artifacts.Configuration
import org.gradle.api.tasks.TaskProvider
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
        extension.androidVariant.convention("debug")
        extension.advanced.toolVersion.convention(KRIT_DEFAULT_VERSION)
        extension.advanced.allRules.convention(false)
        extension.advanced.fixLevel.convention("idiomatic")
        extension.advanced.parallel.convention(Runtime.getRuntime().availableProcessors())
        extension.advanced.noCache.convention(false)
        extension.advanced.typeInference.convention(true)
        extension.advanced.source.setFrom("src/main/kotlin", "src/test/kotlin")
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
            "kritBinaryResolver",
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
            maxParallelUsages.set(1)
        }

        // Shared convention for resolving the krit binary
        val kritBinaryFile = extension.advanced.binary.orElse(
            project.layout.file(project.provider { binaryResolver.get().resolve() })
        )

        val advanced = extension.advanced

        // Wire task defaults for all KritCheckTask instances
        project.tasks.withType(KritCheckTask::class.java).configureEach {
            kritBinary.convention(kritBinaryFile)
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
        }

        // Wire task defaults for all KritFormatTask instances
        project.tasks.withType(KritFormatTask::class.java).configureEach {
            kritBinary.convention(kritBinaryFile)
            config.convention(extension.config)
            fixLevel.convention(advanced.fixLevel)
            parallel.convention(advanced.parallel)
            noCache.convention(advanced.noCache)
            typeInference.convention(advanced.typeInference)
        }

        // Wire task defaults for all KritBaselineTask instances
        project.tasks.withType(KritBaselineTask::class.java).configureEach {
            kritBinary.convention(kritBinaryFile)
            config.convention(extension.config)
            allRules.convention(advanced.allRules)
            parallel.convention(advanced.parallel)
            noCache.convention(advanced.noCache)
            typeInference.convention(advanced.typeInference)
            customRuleJars.from(extension.customRuleJars)
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
        project.plugins.withId("java") {
            project.afterEvaluate {
                // AGP's compile classpath is authoritative if both plugins are present.
                if (!project.plugins.hasPlugin(ANDROID_APPLICATION_PLUGIN_ID) &&
                    !project.plugins.hasPlugin(ANDROID_LIBRARY_PLUGIN_ID)) {
                    val sourceSets = project.extensions.getByType(SourceSetContainer::class.java)
                    val configuration = project.configurations.getByName(JavaPlugin.COMPILE_CLASSPATH_CONFIGURATION_NAME)
                    registerProjectModel(
                        project, extension, "jvm", "main", "",
                        project.provider { sourceSets.getByName("main").allSource.srcDirs.map(::modelPath).sorted() },
                        project.provider { emptyList() }, configuration,
                    )
                }
            }
        }

        listOf(ANDROID_APPLICATION_PLUGIN_ID, ANDROID_LIBRARY_PLUGIN_ID).forEach { pluginId ->
            project.plugins.withId(pluginId) {
                project.afterEvaluate {
                    val candidates = project.configurations.map { it.name }
                        .filter { it.endsWith("CompileClasspath") }
                        .map { it.removeSuffix("CompileClasspath") }
                        .filterNot { it.endsWith("UnitTest") || it.endsWith("AndroidTest") ||
                            it.endsWith("TestFixtures") }
                        .sorted()
                    val variant = pickAndroidVariant(candidates, extension.androidVariant.orNull ?: "debug")
                    val configuration = variant?.let { project.configurations.findByName("${it}CompileClasspath") }
                    val android = project.extensions.findByName("android")
                    val components = project.extensions.findByName("androidComponents")
                    var reason: String? = when {
                        variant == null -> "no *CompileClasspath configuration found"
                        android == null -> "no android extension found"
                        components == null -> "no androidComponents extension found"
                        else -> null
                    }
                    var dirs = emptyList<String>()
                    var boot: Provider<List<String>> = project.provider { emptyList() }
                    if (reason == null) {
                        try {
                            dirs = androidSourceDirs(project, android!!, variant!!)
                        } catch (error: ReflectiveOperationException) {
                            reason = "android.sourceSets reflection failed: ${error.message}"
                        } catch (error: ClassCastException) {
                            reason = "android.sourceSets reflection failed: ${error.message}"
                        }
                    }
                    if (reason == null) {
                        try {
                            val sdk = components!!.javaClass.getMethod("getSdkComponents").invoke(components)
                            @Suppress("UNCHECKED_CAST")
                            val provider = sdk.javaClass.getMethod("getBootClasspath").invoke(sdk)
                                as Provider<List<RegularFile>>
                            boot = provider.map { files -> bootClasspathPaths(files.map { it.asFile }) }
                        } catch (error: ReflectiveOperationException) {
                            reason = "sdkComponents.bootClasspath reflection failed: ${error.message}"
                        } catch (error: ClassCastException) {
                            reason = "sdkComponents.bootClasspath reflection failed: ${error.message}"
                        }
                    }
                    if (reason != null) {
                        project.logger.warn("krit: kritExportModel skipped Android model for ${project.path}: ${reason}")
                    }
                    registerProjectModel(
                        project, extension, "android", variant ?: "", variant ?: "",
                        project.provider { dirs }, boot, if (reason == null) configuration else null,
                    )
                    if (android != null) {
                        candidates.forEach { candidate ->
                            val checkDirs = try {
                                androidSourceDirs(project, android, candidate)
                            } catch (error: ReflectiveOperationException) {
                                project.logger.warn("krit: Android source set reflection failed for ${project.path} $candidate: ${error.message}")
                                emptyList()
                            } catch (error: ClassCastException) {
                                project.logger.warn("krit: Android source set reflection failed for ${project.path} $candidate: ${error.message}")
                                emptyList()
                            }
                            if (checkDirs.isNotEmpty()) {
                                project.tasks.register("kritCheck${candidate.replaceFirstChar(Char::uppercase)}", KritCheckTask::class.java) {
                                    dependsOn(extension.exportModel.map { enabled ->
                                        if (enabled) listOf(project.tasks.named("kritExportModel")) else emptyList<Any>()
                                    })
                                    setSource(project.files(checkDirs))
                                    sourceRoots.from(checkDirs)
                                    description = "Run krit analysis on the '$candidate' variant sources"
                                }
                            }
                        }
                    }
                }
            }
        }
    }

    private fun androidSourceDirs(project: Project, android: Any, variant: String): List<String> {
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
        val names = mutableListOf("main", variant)
        // A trailing capitalized segment is usually the build type; the
        // preceding prefix is often a flavor (or a combined flavor source set).
        Regex("([A-Z][a-z0-9]*)$").find(variant)?.let { match ->
            names.add(match.value.replaceFirstChar(Char::lowercase))
            if (match.range.first > 0) {
                names.add(variant.substring(0, match.range.first).replaceFirstChar(Char::lowercase))
            }
        }
        return names.distinct().flatMap { name ->
            val sourceSet = if (findByName != null) {
                findByName.invoke(sourceSets, name)
            } else {
                try {
                    getByName!!.invoke(sourceSets, name)
                } catch (error: java.lang.reflect.InvocationTargetException) {
                    if (name == "main" ||
                        (error.targetException !is org.gradle.api.UnknownDomainObjectException &&
                            error.targetException !is IllegalArgumentException &&
                            error.targetException !is NoSuchElementException)) throw error
                    null
                }
            }
            if (sourceSet == null) {
                // Only main is required. Variant, build-type, and flavor source
                // sets exist only when declared in the Android DSL.
                if (name == "main") throw ReflectiveOperationException("android.sourceSets has no main source set")
                project.logger.debug("krit: optional Android source set '$name' is absent for ${project.path} $variant")
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

    private fun registerProjectModel(
        project: Project,
        extension: KritExtension,
        platform: String,
        sourceSetName: String,
        variant: String,
        sources: Provider<List<String>>,
        boot: Provider<List<String>>,
        configuration: Configuration?,
    ) {
        val externalArtifacts = configuration?.incoming?.artifactView {
            if (platform == "android") {
                attributes.attribute(ArtifactTypeDefinition.ARTIFACT_TYPE_ATTRIBUTE, "android-classes-jar")
            }
            componentFilter { id -> id !is ProjectComponentIdentifier }
        }?.files
        val dependencies = configuration?.incoming?.resolutionResult?.rootComponent?.map { root ->
            root.dependencies.filterIsInstance<ResolvedDependencyResult>()
                .mapNotNull { (it.selected.id as? ProjectComponentIdentifier)?.projectPath }
                .distinct().sorted()
        }
        val path = project.path
        val fileName = if (path == ":") "_root" else path.removePrefix(":").replace(":", "__")
        val rootPath = modelPath(project.rootDir)
        val export = project.tasks.register("kritExportModel", KritExportModelTask::class.java) {
            description = "Export this project's resolved compile classpath for krit"
            if (externalArtifacts != null) classpath.from(externalArtifacts)
            projectDeps.set(dependencies ?: project.provider { emptyList() })
            sourceDirs.set(if (configuration == null) project.provider { emptyList() } else sources)
            bootClasspath.set(if (configuration == null) project.provider { emptyList() } else boot)
            projectPath.set(path)
            projectDir.set(modelPath(project.projectDir))
            this.platform.set(platform)
            this.variant.set(variant)
            this.sourceSetName.set(sourceSetName)
            hasModel.set(configuration != null)
            generatedBy.set("krit-gradle-plugin ${KritVersion.VERSION}")
            rootDir.set(rootPath)
            outputFile.set(File(rootPath, ".krit/gradle-model/${fileName}.json"))
        }
        project.tasks.named("kritCheck") {
            dependsOn(extension.exportModel.map { enabled ->
                if (enabled) listOf(export) else emptyList<Any>()
            })
        }
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

    companion object {
        /** krit release downloaded when `advanced.toolVersion` is unset: the plugin's own version. */
        const val KRIT_DEFAULT_VERSION = KritVersion.VERSION

        /**
         * Category attribute value identifying a Krit custom-rule bundle
         * variant. Must stay in sync with the matching string in
         * `dev.jasonpearson.krit.custom`'s `KritCustomRulePlugin`.
         */
        const val KRIT_RULE_BUNDLE_CATEGORY = "krit-rule-bundle"

        const val KOTLIN_JVM_PLUGIN_ID = "org.jetbrains.kotlin.jvm"
        const val ANDROID_APPLICATION_PLUGIN_ID = "com.android.application"
        const val ANDROID_LIBRARY_PLUGIN_ID = "com.android.library"

        /**
         * Plugin IDs that trigger krit's per-source-set / per-variant task
         * registration. The settings plugin's auto-application list mirrors
         * this so the two can never drift — adding an entry here should add
         * the same key to the settings plugin and the matching wiring above.
         */
        val LANGUAGE_PLUGIN_IDS = listOf(
            KOTLIN_JVM_PLUGIN_ID,
            ANDROID_APPLICATION_PLUGIN_ID,
            ANDROID_LIBRARY_PLUGIN_ID,
        )
    }
}
