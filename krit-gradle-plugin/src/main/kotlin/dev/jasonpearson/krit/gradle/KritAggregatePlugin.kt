package dev.jasonpearson.krit.gradle

import org.gradle.api.Plugin
import org.gradle.api.Project
import org.gradle.api.attributes.Category

/**
 * Root-project plugin that registers `kritCheckAll`: one krit scan of the whole
 * build with every project's exported Gradle model.
 *
 * Each project with the krit plugin publishes its `kritExportModel` output on a
 * consumable `kritModelElements` variant. This plugin declares the projects to
 * aggregate on the `kritAggregate` configuration and resolves their models
 * through dependency management, the Isolated Projects safe way to share outputs
 * between projects. The settings plugin applies it to the root project and adds
 * every included project automatically; without the settings plugin:
 *
 * ```
 * plugins { id("dev.jasonpearson.krit.aggregate") }
 * dependencies {
 *     kritAggregate(project(":app"))
 *     kritAggregate(project(":lib"))
 * }
 * ```
 *
 * The plugin also applies `dev.jasonpearson.krit`, so the root project's `krit { }`
 * block configures the aggregate scan (config, baseline, reports, binary, FIR,
 * `advanced.moduleShards`). Reports go to `build/reports/krit/all/`.
 */
class KritAggregatePlugin : Plugin<Project> {
    override fun apply(project: Project) {
        project.pluginManager.apply(KritPlugin::class.java)
        val extension = project.extensions.getByType(KritExtension::class.java)

        val declared = project.configurations.create(AGGREGATE_CONFIGURATION) {
            isCanBeConsumed = false
            isCanBeResolved = false
            description = "Projects whose Gradle models kritCheckAll analyzes in one krit scan."
        }
        val models = project.configurations.create("kritAggregateModels") {
            isCanBeConsumed = false
            isCanBeResolved = true
            extendsFrom(declared)
            attributes.attribute(
                Category.CATEGORY_ATTRIBUTE,
                project.objects.named(Category::class.java, KritPlugin.KRIT_GRADLE_MODEL_CATEGORY),
            )
        }
        // Lenient: an included project without the krit plugin (a plain parent
        // directory, a skipped or non-JVM project) has no model variant to select.
        val dependencyModels = models.incoming.artifactView { lenient(true) }.files

        val reportsDir = extension.advanced.reportsDir.map { it.dir("all") }
        val rootDir = project.layout.projectDirectory
        project.tasks.register("kritCheckAll", KritCheckAllTask::class.java) {
            modelFiles.from(dependencyModels)
            // The root project's own model, when it is a JVM / Android / KMP project.
            modelFiles.from(project.tasks.withType(KritExportModelTask::class.java))
            sources.from(rootDir.asFileTree.matching {
                include(KritCheckAllTask.SOURCE_INCLUDES)
                exclude(KritCheckAllTask.SOURCE_EXCLUDES)
            })
            scanRoot.set(rootDir)
            sarifOutput.convention(reportsDir.map { it.file("krit.sarif") })
            jsonOutput.convention(reportsDir.map { it.file("krit.json") })
            plainOutput.convention(reportsDir.map { it.file("krit.txt") })
            checkstyleOutput.convention(reportsDir.map { it.file("krit-checkstyle.xml") })
        }
    }

    companion object {
        /** Dependency-scope configuration listing the projects `kritCheckAll` aggregates. */
        const val AGGREGATE_CONFIGURATION = "kritAggregate"
    }
}
