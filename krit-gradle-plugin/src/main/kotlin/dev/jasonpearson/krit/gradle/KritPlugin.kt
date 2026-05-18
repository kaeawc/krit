package dev.jasonpearson.krit.gradle

import org.gradle.api.Plugin
import org.gradle.api.Project
import org.gradle.api.attributes.Category
import org.gradle.api.plugins.ReportingBasePlugin
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
 *     id("dev.jasonpearson.krit") version "0.1.0"
 * }
 *
 * krit {
 *     toolVersion.set("0.2.0")
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

        val extension = project.extensions.create("krit", KritExtension::class.java, project)

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
        // explicit `customRules(file(...))` entry.
        extension.customRuleJars.from(customRulesConfiguration)

        // Set conventions (defaults)
        extension.ignoreFailures.convention(false)
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

        // Register per-variant tasks for Android projects
        registerAndroidVariantTasks(project, extension)
    }

    /**
     * When the Kotlin JVM plugin is applied, register kritCheck<SourceSet> tasks
     * for each Kotlin source set (e.g., kritCheckMain, kritCheckTest).
     */
    private fun registerKotlinJvmSourceSetTasks(project: Project, extension: KritExtension) {
        project.plugins.withId("org.jetbrains.kotlin.jvm") {
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

    /**
     * When the Android Gradle Plugin is applied (application or library),
     * register kritCheck<Variant> tasks (e.g., kritCheckDebug, kritCheckRelease).
     */
    private fun registerAndroidVariantTasks(project: Project, extension: KritExtension) {
        val androidPluginIds = listOf(
            "com.android.application",
            "com.android.library",
        )

        androidPluginIds.forEach { pluginId ->
            project.plugins.withId(pluginId) {
                project.afterEvaluate {
                    val androidExtension = project.extensions.findByName("android") ?: return@afterEvaluate

                    // Access applicationVariants or libraryVariants via reflection to avoid
                    // a compile-time dependency on the Android Gradle Plugin
                    val variantsPropertyName = when (pluginId) {
                        "com.android.application" -> "getApplicationVariants"
                        "com.android.library" -> "getLibraryVariants"
                        else -> return@afterEvaluate
                    }

                    val variants = try {
                        val method = androidExtension.javaClass.getMethod(variantsPropertyName)
                        @Suppress("UNCHECKED_CAST")
                        method.invoke(androidExtension) as? Iterable<Any>
                    } catch (_: Exception) {
                        null
                    }

                    variants?.forEach { variant ->
                        val variantName = try {
                            variant.javaClass.getMethod("getName").invoke(variant) as String
                        } catch (_: Exception) {
                            return@forEach
                        }

                        // Collect Kotlin source directories for this variant
                        val sourceDirs = try {
                            val sourceSets = variant.javaClass.getMethod("getSourceSets")
                            @Suppress("UNCHECKED_CAST")
                            val sets = sourceSets.invoke(variant) as? Iterable<Any>
                            sets?.flatMap { sourceProvider ->
                                try {
                                    val kotlinDirs = sourceProvider.javaClass
                                        .getMethod("getKotlinDirectories")
                                    @Suppress("UNCHECKED_CAST")
                                    (kotlinDirs.invoke(sourceProvider) as? Iterable<File>)?.toList().orEmpty()
                                } catch (_: Exception) {
                                    // Fall back to Java directories for older AGP
                                    try {
                                        val javaDirs = sourceProvider.javaClass
                                            .getMethod("getJavaDirectories")
                                        @Suppress("UNCHECKED_CAST")
                                        (javaDirs.invoke(sourceProvider) as? Iterable<File>)?.toList().orEmpty()
                                    } catch (_: Exception) {
                                        emptyList()
                                    }
                                }
                            } ?: emptyList()
                        } catch (_: Exception) {
                            emptyList<File>()
                        }

                        if (sourceDirs.isNotEmpty()) {
                            val taskName = "kritCheck${variantName.replaceFirstChar { it.uppercase() }}"
                            if (project.tasks.findByName(taskName) == null) {
                                project.tasks.register(taskName, KritCheckTask::class.java) {
                                    setSource(project.files(sourceDirs))
                                    sourceRoots.from(sourceDirs)
                                    description =
                                        "Run krit analysis on the '$variantName' variant sources"
                                }
                            }
                        }
                    }
                }
            }
        }
    }

    companion object {
        const val KRIT_DEFAULT_VERSION = "0.2.0"

        /**
         * Category attribute value identifying a Krit custom-rule bundle
         * variant. Must stay in sync with the matching string in
         * `dev.jasonpearson.krit.custom`'s `KritCustomRulePlugin`.
         */
        const val KRIT_RULE_BUNDLE_CATEGORY = "krit-rule-bundle"
    }
}
