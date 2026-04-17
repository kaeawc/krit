package dev.krit.gradle

import org.gradle.api.Plugin
import org.gradle.api.Project
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
 *     id("dev.krit") version "0.1.0"
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

        val extension = project.extensions.create("krit", KritExtension::class.java)

        // Set conventions (defaults)
        extension.toolVersion.convention(KRIT_DEFAULT_VERSION)
        extension.allRules.convention(false)
        extension.ignoreFailures.convention(false)
        extension.fixLevel.convention("idiomatic")
        extension.parallel.convention(Runtime.getRuntime().availableProcessors())
        extension.noCache.convention(false)
        extension.typeInference.convention(true)
        extension.source.setFrom("src/main/kotlin", "src/test/kotlin")
        extension.reportsDir.convention(
            project.layout.buildDirectory.dir("reports/krit")
        )

        // Set report conventions
        val reportsDir = extension.reportsDir
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
        ) { spec ->
            spec.parameters.version.set(extension.toolVersion)
            spec.parameters.cacheDir.set(
                project.layout.dir(
                    project.provider {
                        File(System.getProperty("user.home"), ".gradle/krit")
                    }
                )
            )
            spec.maxParallelUsages.set(1)
        }

        // Shared convention for resolving the krit binary
        val kritBinaryFile = extension.binary.orElse(
            project.layout.file(project.provider { binaryResolver.get().resolve() })
        )

        // Wire task defaults for all KritCheckTask instances
        project.tasks.withType(KritCheckTask::class.java).configureEach { task ->
            task.kritBinary.convention(kritBinaryFile)
            task.allRules.convention(extension.allRules)
            task.ignoreFailures.convention(extension.ignoreFailures)
            task.config.convention(extension.config)
            task.baseline.convention(extension.baseline)
            task.parallel.convention(extension.parallel)
            task.noCache.convention(extension.noCache)
            task.typeInference.convention(extension.typeInference)
            // Wire reports from extension
            task.sarifRequired.convention(extension.reports.sarif.required)
            task.sarifOutput.convention(extension.reports.sarif.outputLocation)
            task.jsonRequired.convention(extension.reports.json.required)
            task.jsonOutput.convention(extension.reports.json.outputLocation)
            task.plainRequired.convention(extension.reports.plain.required)
            task.plainOutput.convention(extension.reports.plain.outputLocation)
            task.checkstyleRequired.convention(extension.reports.checkstyle.required)
            task.checkstyleOutput.convention(extension.reports.checkstyle.outputLocation)
        }

        // Wire task defaults for all KritFormatTask instances
        project.tasks.withType(KritFormatTask::class.java).configureEach { task ->
            task.kritBinary.convention(kritBinaryFile)
            task.config.convention(extension.config)
            task.fixLevel.convention(extension.fixLevel)
            task.parallel.convention(extension.parallel)
            task.noCache.convention(extension.noCache)
            task.typeInference.convention(extension.typeInference)
        }

        // Wire task defaults for all KritBaselineTask instances
        project.tasks.withType(KritBaselineTask::class.java).configureEach { task ->
            task.kritBinary.convention(kritBinaryFile)
            task.config.convention(extension.config)
            task.allRules.convention(extension.allRules)
            task.parallel.convention(extension.parallel)
            task.noCache.convention(extension.noCache)
            task.typeInference.convention(extension.typeInference)
        }

        // Register the aggregate kritCheck task
        project.tasks.register("kritCheck", KritCheckTask::class.java) { task ->
            task.setSource(extension.source)
            task.description = "Run krit analysis on all Kotlin sources"
        }

        // Register the kritFormat task
        project.tasks.register("kritFormat", KritFormatTask::class.java) { task ->
            task.source.setFrom(extension.source)
            task.description = "Apply krit auto-fixes to Kotlin sources"
        }

        // Register the kritBaseline task
        project.tasks.register("kritBaseline", KritBaselineTask::class.java) { task ->
            task.source.setFrom(extension.source)
            task.baselineFile.convention(
                project.layout.buildDirectory.file("reports/krit/baseline.xml")
            )
            task.description = "Create a krit baseline file from current findings"
        }

        // Wire kritCheck into the check lifecycle if available
        project.plugins.withType(org.gradle.language.base.plugins.LifecycleBasePlugin::class.java) {
            project.tasks.named("check") { it.dependsOn("kritCheck") }
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
                                project.tasks.register(taskName, KritCheckTask::class.java) { task ->
                                    task.setSource(project.files(kotlinDirs))
                                    task.description = "Run krit analysis on the '$name' source set"
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
                                project.tasks.register(taskName, KritCheckTask::class.java) { task ->
                                    task.setSource(project.files(sourceDirs))
                                    task.description =
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
    }
}
