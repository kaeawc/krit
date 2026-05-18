package dev.jasonpearson.krit.custom

import org.gradle.api.Plugin
import org.gradle.api.Project
import org.gradle.api.attributes.Category
import org.gradle.api.file.DuplicatesStrategy
import org.gradle.api.plugins.JavaPluginExtension
import org.gradle.api.tasks.SourceSet
import org.gradle.api.tasks.bundling.Jar

/**
 * Scaffolds a module for authoring custom Krit rules. See the plugin README
 * for the consumer-facing walkthrough.
 */
class KritCustomRulePlugin : Plugin<Project> {

    override fun apply(project: Project) {
        project.pluginManager.apply("org.jetbrains.kotlin.jvm")

        val extension = project.extensions.create(
            "kritCustomRules",
            KritCustomRulesExtension::class.java,
        )
        extension.ruleApiVersion.convention(BuildConfig.DEFAULT_RULE_API_VERSION)
        extension.sdkVersion.convention(extension.ruleApiVersion)
        extension.vendorId.convention("custom")
        extension.defaultSeverity.convention("warning")

        project.configurations.named("implementation").configure {
            dependencies.addLater(
                extension.ruleApiVersion.map { v ->
                    project.dependencies.create("dev.jasonpearson.krit:krit-rule-api:$v")
                }
            )
        }

        // PSI types are surfaced on the rule API but provided at
        // runtime by the krit-types daemon. Consumers must add the
        // JetBrains intellij-dependencies redirector to settings —
        // see docs/external-rules.md.
        project.configurations.named("compileOnly").configure {
            dependencies.add(
                project.dependencies.create(
                    "org.jetbrains.kotlin:kotlin-compiler:$KOTLIN_COMPILER_VERSION",
                ),
            )
        }

        val javaExtension = project.extensions.getByType(JavaPluginExtension::class.java)
        val mainSourceSet = javaExtension.sourceSets.getByName(SourceSet.MAIN_SOURCE_SET_NAME)

        val servicesTask = project.tasks.register(
            "generateKritRuleServices",
            KritRuleServicesTask::class.java,
        ) {
            group = "krit"
            description = "Generates META-INF/services for KritRule implementations."
            classesDirs.from(mainSourceSet.output.classesDirs)
            resourcesDirs.from(mainSourceSet.resources.srcDirs)
            outputDir.convention(
                project.layout.buildDirectory.dir("generated/krit/services")
            )
            dependsOn(mainSourceSet.classesTaskName)
        }

        val pluginVersion = BuildConfig.PLUGIN_VERSION
        val kritRuleJarTask = project.tasks.register(
            "kritRuleJar",
            Jar::class.java,
        ) {
            group = "krit"
            description = "Builds a Krit custom-rule jar."
            archiveClassifier.set("krit-rules")
            from(mainSourceSet.output)
            from(servicesTask.flatMap { it.outputDir })
            duplicatesStrategy = DuplicatesStrategy.EXCLUDE
            dependsOn(servicesTask)
            // Providers, not .get() — late `kritCustomRules { ... }` overrides
            // must still flow through, and configuration cache requires lazy
            // resolution.
            manifest {
                attributes(
                    mapOf(
                        MANIFEST_SDK_VERSION to extension.sdkVersion,
                        MANIFEST_PLUGIN_VERSION to pluginVersion,
                        MANIFEST_VENDOR_ID to extension.vendorId,
                        MANIFEST_DEFAULT_SEVERITY to extension.defaultSeverity,
                    )
                )
            }
        }

        // Outgoing variant consumed by the host plugin's `kritCustomRules`
        // resolvable configuration. The Category attribute uses a Krit-specific
        // value so this bundle is fully decoupled from `runtimeElements` /
        // `apiElements` — projects expressing
        //   `dependencies { kritCustomRules(project(":this")) }`
        // pull the stamped jar directly, without intercepting normal compile
        // or runtime classpaths.
        val kritRuleBundleCategory = project.objects.named(
            Category::class.java,
            KRIT_RULE_BUNDLE_CATEGORY,
        )
        project.configurations.create("kritRuleBundleElements") {
            isCanBeConsumed = true
            isCanBeResolved = false
            description = "Krit custom-rule bundle artifact produced by this module."
            attributes.attribute(Category.CATEGORY_ATTRIBUTE, kritRuleBundleCategory)
            outgoing.artifact(kritRuleJarTask.flatMap { it.archiveFile }) {
                builtBy(kritRuleJarTask)
            }
        }
    }

    internal companion object {
        internal const val MANIFEST_SDK_VERSION = "Krit-SDK-Version"
        internal const val MANIFEST_PLUGIN_VERSION = "Krit-Plugin-Version"
        internal const val MANIFEST_VENDOR_ID = "Krit-Vendor-Id"
        internal const val MANIFEST_DEFAULT_SEVERITY = "Krit-Default-Severity"

        /**
         * Category attribute value identifying a Krit custom-rule bundle
         * variant. Must stay in sync with the matching string in
         * `dev.jasonpearson.krit`'s `KritPlugin`.
         */
        internal const val KRIT_RULE_BUNDLE_CATEGORY = "krit-rule-bundle"

        // Must track the krit-types daemon's bundled compiler so PSI
        // classes resolve at runtime.
        internal const val KOTLIN_COMPILER_VERSION = "2.3.21"
    }
}
