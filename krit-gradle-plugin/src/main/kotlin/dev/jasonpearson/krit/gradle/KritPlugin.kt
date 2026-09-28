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
import org.gradle.api.tasks.compile.JavaCompile
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
        project.afterEvaluate {
            val kmp = project.plugins.hasPlugin(KOTLIN_MULTIPLATFORM_PLUGIN_ID)
            val android = project.plugins.hasPlugin(ANDROID_APPLICATION_PLUGIN_ID) ||
                project.plugins.hasPlugin(ANDROID_LIBRARY_PLUGIN_ID)
            val specs = when {
                kmp -> {
                    val kmpSpecs = kmpModels(project, extension, android)
                    if (android && kmpSpecs.none { it.platform == "android" })
                        kmpSpecs + androidModels(project, extension) else kmpSpecs
                }
                android -> androidModels(project, extension)
                project.plugins.hasPlugin("java") -> jvmModels(project)
                else -> return@afterEvaluate
            }
            registerProjectModel(project, extension, specs)
        }
    }

    private data class ModelSpec(
        val name: String, val kind: String, val platform: String, val variant: String,
        val dirs: List<String>, val boot: Provider<List<String>>, val configuration: Configuration,
        val compileTask: String,
    )

    private fun selectedAndroidVariant(project: Project, extension: KritExtension): String? =
        pickAndroidVariant(project.configurations.map { it.name }
            .filter { it.endsWith("CompileClasspath") }
            .map { it.removeSuffix("CompileClasspath") }
            .filterNot { it.endsWith("UnitTest") || it.endsWith("AndroidTest") ||
                it.endsWith("TestFixtures") }, extension.androidVariant.orNull ?: "debug")

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
            ModelSpec(name, name, "jvm", "",
                sourceSets.getByName(name).allSource.srcDirs.map(::modelPath), empty, config,
                if (name == "main") "compileKotlin" else "compile${name.replaceFirstChar(Char::uppercase)}Kotlin")
        }
    }

    private fun androidModels(project: Project, extension: KritExtension): List<ModelSpec> {
        val candidates = project.configurations.map { it.name }
            .filter { it.endsWith("CompileClasspath") }
            .map { it.removeSuffix("CompileClasspath") }
            .filterNot { it.endsWith("UnitTest") || it.endsWith("AndroidTest") ||
                it.endsWith("TestFixtures") }.sorted()
        val variant = selectedAndroidVariant(project, extension)
        val android = project.extensions.findByName("android")
        val components = project.extensions.findByName("androidComponents")
        if (variant == null || android == null || components == null) {
            project.logger.warn("krit: kritExportModel skipped Android model for ${project.path}: missing variant or Android extension")
            return emptyList()
        }
        return try {
            val boot = androidBootClasspath(project)
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
                        boot, config, "compile${name.replaceFirstChar(Char::uppercase)}Kotlin")
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
            specs
        } catch (error: Exception) {
            project.logger.warn("krit: kritExportModel skipped Android model for ${project.path}: ${error.message}")
            emptyList()
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
                val variant = if (platform == "android") selectedAndroidVariant(project, extension) else null
                val requested = if (platform == "android") {
                    if (variant == null) {
                        project.logger.warn("krit: KMP Android variant absent for ${project.path} target $targetName")
                        return@forEach
                    }
                    listOf(variant to "main", "${variant}UnitTest" to "test",
                        "${variant}AndroidTest" to "androidTest")
                } else listOf("main" to "main", "test" to "test")
                val boot = if (platform == "android" && android) androidBootClasspath(project)
                    else project.provider { emptyList<String>() }
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
                    val sourceSets = reflect(compilation, "getKotlinSourceSets") as Iterable<*>
                    val dirs = sourceSets.filterNotNull().flatMap { sourceSet ->
                        val kotlinDirs = reflect(sourceSet, "getKotlin")
                        @Suppress("UNCHECKED_CAST")
                        (reflect(kotlinDirs, "getSrcDirs") as Collection<File>).map(::modelPath)
                    }.distinct().sorted()
                    val name = "${targetName}${compilationName.replaceFirstChar(Char::uppercase)}"
                    val compileTask = runCatching {
                        reflect(compilation, "getCompileKotlinTaskName").toString()
                    }.getOrDefault("compileKotlin${name.replaceFirstChar(Char::uppercase)}")
                    targetSpecs += ModelSpec(name, kind, platform, variant ?: name, dirs,
                        boot, configuration, compileTask)
                }
                specs += targetSpecs
            } catch (error: Exception) {
                project.logger.warn("krit: KMP model skipped target $targetName in ${project.path}: ${error.message}")
            }
        }
        return specs
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

    private fun androidRJar(project: Project, variant: String, kind: String, major: Int): Provider<List<String>> {
        val libraryMain = kind == "main" && project.plugins.hasPlugin(ANDROID_LIBRARY_PLUGIN_ID)
        val runtimeFolders = listOf("compile_and_runtime_not_namespaced_r_class_jar",
            "compile_and_runtime_r_class_jar")
        val folders = if (libraryMain) listOf("compile_r_class_jar") else runtimeFolders
        val predictedFolder = if (libraryMain) "compile_r_class_jar"
            else if (major == 8) runtimeFolders[0] else runtimeFolders[1]
        val directory = when (kind) {
            "test" -> "${variant}UnitTest"
            "androidTest" -> "${variant}AndroidTest"
            else -> variant
        }
        val capitalized = directory.replaceFirstChar(Char::uppercase)
        val task = when (kind) {
            "test" -> "generate${capitalized}StubRFile"
            "androidTest" -> "process${capitalized}Resources"
            else -> if (libraryMain) "generate${capitalized}RFile" else "process${capitalized}Resources"
        }
        val buildDirectory = project.layout.buildDirectory
        return project.provider {
            val intermediates = buildDirectory.get().asFile.resolve("intermediates")
            val found = folders.flatMap { folder ->
                intermediates.resolve(folder).resolve(directory).listFiles()
                    ?.filter { it.isDirectory }
                    ?.map { it.resolve("R.jar") }
                    ?.filter { it.isFile }
                    ?: emptyList()
            }.map(::modelPath).distinct().sorted()
            found.ifEmpty {
                listOf(modelPath(intermediates.resolve(predictedFolder).resolve(directory)
                    .resolve(task).resolve("R.jar")))
            }
        }
    }

    private fun registerProjectModel(project: Project, extension: KritExtension, specs: List<ModelSpec>) {
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
        if (androidSpecs.isNotEmpty() && resolvedMajor == null)
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
                sourceDirs.set(spec.dirs)
                bootClasspath.set(spec.boot)
                jvmTarget.set(jvmTarget(project, spec.compileTask, spec.kind))
                val externalArtifacts = spec.configuration.incoming.artifactView {
                    if (spec.platform == "android") {
                        attributes.attribute(ArtifactTypeDefinition.ARTIFACT_TYPE_ATTRIBUTE, "android-classes-jar")
                    }
                    componentFilter { id -> id !is ProjectComponentIdentifier }
                }.files
                classpath.from(externalArtifacts)
                projectDeps.set(spec.configuration.incoming.resolutionResult.rootComponent.map { root ->
                    root.dependencies.filterIsInstance<ResolvedDependencyResult>()
                        .mapNotNull { (it.selected.id as? ProjectComponentIdentifier)?.projectPath }
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
            outputFile.set(File(rootPath, ".krit/gradle-model/${fileName}.json"))
        }
        if (extension.exportGenerated.get()) {
            val variantNames = specs.filter { it.platform == "android" }.map { it.variant }
                .filter { it.isNotEmpty() }.distinct()
            val androidNames = variantNames.flatMap { variant ->
                val suffix = variant.replaceFirstChar(Char::uppercase)
                listOf("generate${suffix}RFile", "process${suffix}Resources")
            }
            val relevant = specs.map { it.name.lowercase() } +
                variantNames.map { it.lowercase() } +
                specs.mapNotNull { spec ->
                    Regex("^[a-z0-9]+(?=[A-Z])").find(spec.name)?.value?.lowercase()
                } +
                specs.filter { it.name == "main" }.map { "kotlin" }
            val hasKsp = project.plugins.hasPlugin("com.google.devtools.ksp")
            val hasKapt = project.plugins.hasPlugin("org.jetbrains.kotlin.kapt") ||
                project.plugins.hasPlugin("kotlin-kapt")
            val generators = project.tasks.names.filter { taskName ->
                val lower = taskName.lowercase()
                taskName in androidNames ||
                    (((hasKsp && lower.startsWith("ksp")) ||
                        (hasKapt && lower.startsWith("kapt"))) &&
                        (lower.contains("kotlin") || lower.contains("generatestubs")) &&
                        relevant.any { lower.contains(it) })
            }
            export.configure { dependsOn(generators) }
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
        const val KOTLIN_MULTIPLATFORM_PLUGIN_ID = "org.jetbrains.kotlin.multiplatform"
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
            KOTLIN_MULTIPLATFORM_PLUGIN_ID,
            ANDROID_APPLICATION_PLUGIN_ID,
            ANDROID_LIBRARY_PLUGIN_ID,
        )
    }
}
