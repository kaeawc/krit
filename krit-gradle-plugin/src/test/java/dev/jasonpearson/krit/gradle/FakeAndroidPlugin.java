package dev.jasonpearson.krit.gradle;

import java.io.File;
import java.util.List;
import java.util.Set;
import org.gradle.api.Plugin;
import org.gradle.api.Project;
import org.gradle.api.Action;
import org.gradle.api.DefaultTask;
import org.gradle.api.file.Directory;
import org.gradle.api.file.DirectoryProperty;
import org.gradle.api.tasks.OutputDirectory;
import org.gradle.api.tasks.TaskAction;
import org.gradle.api.tasks.TaskProvider;
import org.gradle.api.file.RegularFile;
import org.gradle.api.provider.ListProperty;
import org.gradle.api.provider.Property;
import org.gradle.api.provider.Provider;

/** Minimal Android-shaped plugin for TestKit's plugin-ID and model export tests. */
public class FakeAndroidPlugin implements Plugin<Project> {
    // Mirrors AndroidArtifacts.ArtifactType.R_CLASS_JAR = "r-class-jar" in AGP 8.8.0-alpha09.
    public static final String DEPENDENCY_R_CLASS_JAR_ARTIFACT_TYPE = "r-class-jar";
    /** Mirrors org.jetbrains.kotlin.gradle.dsl.KotlinVersion: the "2.0" spelling is getVersion(). */
    public static class FakeKotlinVersion {
        private final String version;
        public FakeKotlinVersion(String version) { this.version = version; }
        public String getVersion() { return version; }
        @Override public String toString() { return "KOTLIN_" + version.replace('.', '_'); }
    }
    public static class CompilerOptions {
        private final Property<String> jvmTarget;
        private final Property<FakeKotlinVersion> languageVersion;
        private final Property<FakeKotlinVersion> apiVersion;
        private final ListProperty<String> optIn;
        private final ListProperty<String> freeCompilerArgs;
        private final Property<Boolean> progressiveMode;
        public CompilerOptions(Project project) {
            jvmTarget = project.getObjects().property(String.class);
            jvmTarget.set("17");
            languageVersion = project.getObjects().property(FakeKotlinVersion.class);
            apiVersion = project.getObjects().property(FakeKotlinVersion.class);
            optIn = project.getObjects().listProperty(String.class);
            freeCompilerArgs = project.getObjects().listProperty(String.class);
            progressiveMode = project.getObjects().property(Boolean.class);
            // Kotlin settings are only configured when a test asks for them, so the
            // other fixtures keep exercising the "nothing configured" export.
            if (project.hasProperty("fakeKotlinOptions")) {
                languageVersion.set(new FakeKotlinVersion("1.9"));
                apiVersion.set(new FakeKotlinVersion("1.8"));
                optIn.add("kotlin.RequiresOptIn");
                optIn.add("com.example.Experimental");
                freeCompilerArgs.addAll("-Xcontext-parameters", "-Xexplicit-backing-fields");
                progressiveMode.set(true);
            }
        }
        public Property<String> getJvmTarget() { return jvmTarget; }
        public Property<FakeKotlinVersion> getLanguageVersion() { return languageVersion; }
        public Property<FakeKotlinVersion> getApiVersion() { return apiVersion; }
        public ListProperty<String> getOptIn() { return optIn; }
        public ListProperty<String> getFreeCompilerArgs() { return freeCompilerArgs; }
        public Property<Boolean> getProgressiveMode() { return progressiveMode; }
    }
    /** Mirrors KotlinTopLevelExtension.getCoreLibrariesVersion(). */
    public static class FakeKotlinExtension {
        // User-overridable and here pinned below the plugin version, as in real builds.
        public String getCoreLibrariesVersion() { return "2.0.0"; }
    }
    /** Mirrors org.jetbrains.kotlin.gradle.plugin.SubpluginOption: a key and a lazily computed value. */
    public static class FakeSubpluginOption {
        private final String key;
        private final String value;
        public FakeSubpluginOption(String key, String value) { this.key = key; this.value = value; }
        public String getKey() { return key; }
        public String getValue() { return value; }
    }
    /** Mirrors org.jetbrains.kotlin.gradle.plugin.CompilerPluginConfig.allOptions(). */
    public static class FakeCompilerPluginConfig {
        private final java.util.Map<String, List<FakeSubpluginOption>> options = new java.util.LinkedHashMap<>();
        public java.util.Map<String, List<FakeSubpluginOption>> allOptions() { return options; }
    }
    public static class CompileKotlin extends DefaultTask {
        private final CompilerOptions compilerOptions = new CompilerOptions(getProject());
        private final org.gradle.api.file.ConfigurableFileCollection pluginClasspath = getProject().getObjects().fileCollection();
        private final ListProperty<FakeCompilerPluginConfig> pluginOptions =
                getProject().getObjects().listProperty(FakeCompilerPluginConfig.class);
        public CompileKotlin() {
            // Mirrors AbstractKotlinCompile.getPluginClasspath()/getPluginOptions() in KGP 2.x: the applied plugins
            // after KGP's own scripting support.
            if (getProject().hasProperty("fakeCompilerPlugins")) {
                pluginClasspath.from(
                        getProject().file("plugins/kotlin-scripting-compiler-embeddable-2.4.21.jar"),
                        getProject().file("plugins/kotlin-serialization-compiler-plugin-embeddable-2.4.21.jar"));
                FakeCompilerPluginConfig config = new FakeCompilerPluginConfig();
                config.allOptions().put("org.jetbrains.kotlinx.serialization",
                        List.of(new FakeSubpluginOption("disableIntrinsic", "true")));
                config.allOptions().put("org.jetbrains.kotlin.allopen",
                        List.of(new FakeSubpluginOption("annotation", "a.Open"), new FakeSubpluginOption("annotation", "b.Open")));
                pluginOptions.add(config);
            }
        }
        public CompilerOptions getCompilerOptions() { return compilerOptions; }
        public org.gradle.api.file.ConfigurableFileCollection getPluginClasspath() { return pluginClasspath; }
        public ListProperty<FakeCompilerPluginConfig> getPluginOptions() { return pluginOptions; }
    }
    public static class Dirs {
        private final Set<File> dirs;
        public Dirs(File dir) { dirs = Set.of(dir); }
        public Set<File> getSrcDirs() { return dirs; }
    }

    public static class Source {
        private final Dirs java;
        private final Dirs kotlin;
        public Source(File root, String name) {
            java = new Dirs(new File(root, "src/" + name + "/java"));
            kotlin = new Dirs(new File(root, "src/" + name + "/kotlin"));
        }
        public Dirs getJava() { return java; }
        public Dirs getKotlin() { return kotlin; }
    }

    public static class Sources {
        private final File root;
        public Sources(File root) { this.root = root; }
        public Source findByName(String name) {
            // Mirrors BaseExtension.getSourceSets().findByName in AGP 8.8.0-alpha09.
            return Set.of("main", "debug", "release", "test", "testDebug", "testStaging",
                    "androidTest", "androidTestDebug", "androidTestStaging").contains(name)
                    ? new Source(root, name) : null;
        }
        public Source getByName(String name) {
            Source source = findByName(name);
            if (source == null) throw new IllegalArgumentException(name);
            return source;
        }
    }

    public static class Android {
        private final Sources sources;
        private final String targetProjectPath;
        public Android(File root, String targetProjectPath) {
            sources = new Sources(root);
            this.targetProjectPath = targetProjectPath;
        }
        public Sources getSourceSets() { return sources; }
        // Mirrors TestExtension.getTargetProjectPath() in AGP 8.x/9.x (null until configured).
        public String getTargetProjectPath() { return targetProjectPath; }
    }

    /** True for a project listed in -PfakeTestModules: com.android.test's layout, with only main and no test components. */
    static boolean isTestModule(Project project) {
        Object modules = project.findProperty("fakeTestModules");
        return modules != null && List.of(modules.toString().split(",")).contains(project.getName());
    }

    public static class Sdk {
        private final Provider<List<RegularFile>> boot;
        public Sdk(Project project) {
            boot = project.provider(() -> List.of(project.getLayout().getProjectDirectory().file("android.jar")));
        }
        // Mirrors SdkComponents.getBootClasspath() in gradle-api:8.8.0-alpha09.
        public Provider<List<RegularFile>> getBootClasspath() { return boot; }
    }

    /** Mirrors VariantSelector in AGP 8.x/9.x. */
    public static class Selector { public Selector all() { return this; } }

    /** Mirrors SourceDirectories.Flat: getAll() is every root, generated ones with their producer tasks. */
    public static class Directories {
        private final Provider<List<Directory>> all;
        public Directories(Provider<List<Directory>> all) { this.all = all; }
        public Provider<List<Directory>> getAll() { return all; }
    }
    /** Mirrors com.android.build.api.variant.Sources (java and kotlin are nullable). */
    public static class VariantSources {
        private final Directories java;
        private final Directories kotlin;
        public VariantSources(Directories java, Directories kotlin) { this.java = java; this.kotlin = kotlin; }
        public Directories getJava() { return java; }
        public Directories getKotlin() { return kotlin; }
    }
    /** Mirrors a Variant or test Component: getName(), getSources(), and for variants getUnitTest(). */
    public static class Component {
        private final String name;
        private final VariantSources sources;
        private final Component unitTest;
        public Component(String name, VariantSources sources, Component unitTest) {
            this.name = name; this.sources = sources; this.unitTest = unitTest;
        }
        public String getName() { return name; }
        public VariantSources getSources() { return sources; }
        public Component getUnitTest() { return unitTest; }
    }

    /** A source generator registered the way SQLDelight, Wire or KSP register theirs: an output directory. */
    public abstract static class FakeSourceGenerator extends DefaultTask {
        @OutputDirectory public abstract DirectoryProperty getOutputDir();
        @TaskAction public void generate() throws java.io.IOException {
            File dir = new File(getOutputDir().get().getAsFile(), "com/example");
            dir.mkdirs();
            java.nio.file.Files.writeString(new File(dir, "Generated.kt").toPath(), "package com.example\nclass Generated\n");
        }
    }

    public static class Components {
        private final Sdk sdk;
        private final List<Action<Object>> variantActions = new java.util.ArrayList<>();
        public Components(Project project) { sdk = new Sdk(project); }
        public Sdk getSdkComponents() { return sdk; }
        // Mirrors AndroidComponentsExtension.selector() and onVariants(VariantSelector, Action) in AGP 8.x/9.x.
        public Selector selector() { return new Selector(); }
        public void onVariants(Selector selector, Action<Object> action) { variantActions.add(action); }
        void runVariantActions(Object variant) { variantActions.forEach(action -> action.execute(variant)); }
        // Mirrors AndroidComponents.getPluginVersion().getMajor() in AGP 8.8.0-alpha09.
        public Version getPluginVersion() { return new Version(); }
    }
    public static class Version { public int getMajor() { return 8; } }

    @Override public void apply(Project project) {
        Object target = isTestModule(project) ? project.findProperty("fakeTargetProjectPath") : null;
        project.getExtensions().add("android", new Android(project.getProjectDir(), target == null ? null : target.toString()));
        if (project.hasProperty("fakeKotlinOptions"))
            project.getExtensions().add("kotlin", new FakeKotlinExtension());
        if (!project.hasProperty("omitComponents"))
            project.getExtensions().add("androidComponents", new Components(project));
        if (project.hasProperty("fakeRJarKind")) {
            String kind = project.property("fakeRJarKind").toString();
            String folder = kind.equals("main") ? "compile_r_class_jar"
                    : "compile_and_runtime_not_namespaced_r_class_jar";
            String directory = kind.equals("main") ? "debug"
                    : kind.equals("test") ? "debugUnitTest" : "debugAndroidTest";
            String task = kind.equals("main") ? "generateDebugRFile"
                    : kind.equals("test") ? "generateDebugUnitTestStubRFile"
                    : "processDebugAndroidTestResources";
            File jar = new File(project.getLayout().getBuildDirectory().get().getAsFile(),
                    "intermediates/" + folder + "/" + directory + "/" + task + "/R.jar");
            jar.getParentFile().mkdirs();
            try { jar.createNewFile(); }
            catch (java.io.IOException error) { throw new RuntimeException(error); }
        }
        if (project.hasProperty("deferAndroidVariants")) {
            project.afterEvaluate(ignored -> registerVariants(project));
        } else {
            registerVariants(project);
        }
    }

    private void registerVariants(Project project) {
        if (project.hasProperty("omitAndroidVariants")) return;
        // Mirrors TestApplicationTaskManager in AGP 8.x/9.x: a com.android.test module has one
        // component per build type and no unit-test or androidTest components.
        List<String> configurations = project.hasProperty("releaseOnly")
                ? List.of("releaseCompileClasspath")
                : isTestModule(project) ? List.of("debugCompileClasspath", "releaseCompileClasspath")
                : List.of("debugCompileClasspath", "releaseCompileClasspath",
                "stagingDebugCompileClasspath",
                "debugUnitTestCompileClasspath", "debugAndroidTestCompileClasspath",
                "stagingDebugUnitTestCompileClasspath", "stagingDebugAndroidTestCompileClasspath");
        for (String name : configurations) {
            project.getConfigurations().create(name).setCanBeResolved(true);
        }
        // Mirrors GenerateLibraryRFileTask.CreationAction.getName() and LinkApplicationAndroidResourcesTask.BaseCreationAction.getName() in AGP 8.8.0-alpha09.
        // The unit-test and androidTest R tasks mirror GenerateLibraryRFileTask.TestRuntimeStubRClassCreationAction
        // and ProcessTestManifest's resource linking in AGP 8.x.
        for (String name : List.of("generateDebugRFile", "processDebugResources",
                "generateDebugUnitTestStubRFile", "processDebugAndroidTestResources"))
            project.getTasks().register(name);
        project.getTasks().register("compileDebugKotlin", CompileKotlin.class);
        Object components = project.getExtensions().findByName("androidComponents");
        if (project.hasProperty("fakeGeneratedSources") && components instanceof Components) {
            TaskProvider<FakeSourceGenerator> generator = project.getTasks().register(
                    "generateDebugFakeSources", FakeSourceGenerator.class,
                    task -> task.getOutputDir().set(project.getLayout().getBuildDirectory().dir("generated/source/fake/debug")));
            // Like AGP's providers, these capture directories, never the Project (configuration cache).
            Directory mainJava = project.getLayout().getProjectDirectory().dir("src/main/java");
            Directory mainKotlin = project.getLayout().getProjectDirectory().dir("src/main/kotlin");
            // Shaped like AGP's flat source directories: a list property whose generated element is the
            // producer's output property, readable as a location and carrying the task dependency.
            ListProperty<Directory> main = project.getObjects().listProperty(Directory.class);
            main.add(mainJava);
            main.add(generator.flatMap(FakeSourceGenerator::getOutputDir));
            Provider<List<Directory>> kotlin = project.provider(() -> List.of(mainKotlin));
            Component unitTest = new Component("debugUnitTest", new VariantSources(null, null), null);
            // AGP runs onVariants callbacks while it creates each variant.
            ((Components) components).runVariantActions(
                    new Component("debug", new VariantSources(new Directories(main), new Directories(kotlin)), unitTest));
        }
    }
}
