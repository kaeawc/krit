package dev.jasonpearson.krit.gradle;

import java.io.File;
import java.util.ArrayList;
import java.util.List;
import java.util.Map;
import java.util.Set;
import org.gradle.api.Plugin;
import org.gradle.api.Project;

/** KGP-shaped fixture with no dependency on the real Kotlin Gradle plugin. */
public class FakeKmpPlugin implements Plugin<Project> {
    public static class Dirs {
        private final Set<File> dirs;
        public Dirs(File dir) { dirs = Set.of(dir); }
        public Set<File> getSrcDirs() { return dirs; }
    }
    public static class SourceSet {
        private final Dirs kotlin;
        public SourceSet(File root, String name) { this(new File(root, "src/" + name + "/kotlin")); }
        public SourceSet(File dir) { kotlin = new Dirs(dir); }
        public Dirs getKotlin() { return kotlin; }
    }
    /** Mirrors AGP 9.5.3's KotlinMultiplatformAndroidHostTestCompilationImpl / DeviceTestCompilationImpl. */
    public static class HostTestCompilation extends Compilation
            implements com.android.build.api.dsl.KotlinMultiplatformAndroidHostTestCompilation {
        public HostTestCompilation(File root, String name) {
            super(root, "android", name, "androidHostTestCompileClasspath", "compileAndroidHostTest");
        }
    }
    public static class DeviceTestCompilation extends Compilation
            implements com.android.build.api.dsl.KotlinMultiplatformAndroidDeviceTestCompilation {
        public DeviceTestCompilation(File root, String name) {
            super(root, "android", name, "androidDeviceTestCompileClasspath", "compileAndroidDeviceTest");
        }
    }
    public static class Compilation {
        private final File root;
        private final String target;
        private final String name;
        private final String config;
        private final String compileTask;
        private final List<SourceSet> sourceSets;
        private final List<SourceSet> allSourceSets = new ArrayList<>();
        public Compilation(File root, String target, String name) {
            this(root, target, name, target + capitalize(name) + "CompileClasspath", null);
        }
        public Compilation(File root, String target, String name, String config, String compileTask) {
            this.root = root;
            this.target = target;
            this.name = name;
            this.config = config;
            this.compileTask = compileTask;
            sourceSets = List.of(new SourceSet(root, target + capitalize(name)));
            allSourceSets.addAll(sourceSets);
        }
        public String getName() { return name; }
        // Mirrors KotlinCompilationImpl.getCompileKotlinTaskName() and DefaultKotlinCompilationTaskNamesContainerFactory.create() in KGP 2.2.21.
        public String getCompileKotlinTaskName() {
            if (compileTask != null) return compileTask;
            return "compile" + (name.equals("main") ? "" : capitalize(name))
                    + "Kotlin" + capitalize(target);
        }
        public String getCompileDependencyConfigurationName() { return config; }
        // Mirrors KGP 2.4.21: kotlinSourceSets is the compilation's own source set (jvmMain), and
        // allKotlinSourceSets adds its dependsOn closure (commonMain), whose default-hierarchy
        // edges KGP adds only after the project's afterEvaluate callbacks ran (see apply).
        public List<SourceSet> getKotlinSourceSets() { return sourceSets; }
        public List<SourceSet> getAllKotlinSourceSets() { return allSourceSets; }
        void refineCommon() {
            allSourceSets.add(new SourceSet(root, name.toLowerCase().contains("test") ? "commonTest" : "commonMain"));
        }
    }
    static String capitalize(String value) { return value.substring(0, 1).toUpperCase() + value.substring(1); }
    public static class Compilations implements Iterable<Compilation> {
        private final Map<String, Compilation> compilations;
        public Compilations(File root, String target, boolean android, boolean androidKmpLibrary, boolean androidKmpTests) {
            // Mirrors KotlinAndroidTarget.getCompilations() and AndroidProjectHandlerKt.getVariantName() in KGP 2.2.21;
            // the Android-KMP library target (AGP 9.5.3) has one `main` compilation while its host and
            // device tests are not enabled. With them on (-PfakeAndroidKmpTests) it also has hostTest and
            // deviceTest, here plus a compilation of no Android-KMP type (`benchmark`).
            compilations = androidKmpLibrary && androidKmpTests
                    ? Map.of("main", new Compilation(root, target, "main", "androidCompileClasspath", "compileAndroidMain"),
                        "hostTest", new HostTestCompilation(root, "hostTest"),
                        "deviceTest", new DeviceTestCompilation(root, "deviceTest"),
                        "benchmark", new Compilation(root, target, "benchmark", "androidBenchmarkCompileClasspath", "compileAndroidBenchmark"))
                    : androidKmpLibrary
                    ? Map.of("main", new Compilation(root, target, "main", "androidCompileClasspath", "compileAndroidMain"))
                    : android
                    ? Map.of("debug", new Compilation(root, target, "debug"),
                        "debugUnitTest", new Compilation(root, target, "debugUnitTest"),
                        "debugAndroidTest", new Compilation(root, target, "debugAndroidTest"))
                    : Map.of("main", new Compilation(root, target, "main"),
                        "test", new Compilation(root, target, "test"));
        }
        public Compilation findByName(String name) { return compilations.get(name); }
        @Override public java.util.Iterator<Compilation> iterator() { return compilations.values().iterator(); }
        Iterable<Compilation> all() { return compilations.values(); }
    }
    public static class Platform {
        private final String name;
        public Platform(String name) { this.name = name; }
        public String getName() { return name; }
    }
    public static class Target {
        private final String name;
        private final Platform platformType;
        private final Compilations compilations;
        private final Resources androidResources;
        public Target(File root, String name, String platform, boolean androidKmpLibrary, boolean androidKmpTests, boolean resources) {
            this.name = name;
            platformType = new Platform(platform);
            compilations = new Compilations(root, name, platform.equals("androidJvm"), androidKmpLibrary, androidKmpTests);
            androidResources = new Resources(resources);
        }
        public String getName() { return name; }
        // Mirrors KotlinMultiplatformAndroidLibraryExtension.getAndroidResources().getEnable() in AGP 9.5.3.
        public Resources getAndroidResources() { return androidResources; }
        public Platform getPlatformType() { return platformType; }
        public Compilations getCompilations() { return compilations; }
    }
    public static class Resources {
        private final boolean enable;
        public Resources(boolean enable) { this.enable = enable; }
        public boolean getEnable() { return enable; }
    }
    public static class Kotlin {
        private final List<Target> targets;
        private final File root;
        private final boolean generatedCommonRoot;
        public Kotlin(File root, boolean android, boolean androidKmpLibrary, boolean androidKmpTests, boolean resources,
                      boolean generatedCommonRoot) {
            this.root = root;
            this.generatedCommonRoot = generatedCommonRoot;
            targets = android || androidKmpLibrary
                    ? List.of(new Target(root, "android", "androidJvm", androidKmpLibrary, androidKmpTests, resources),
                        new Target(root, "jvm", "jvm", false, false, false))
                    : List.of(new Target(root, "jvm", "jvm", false, false, false), new Target(root, "js", "js", false, false, false));
        }
        public List<Target> getTargets() { return targets; }
        // Mirrors KotlinMultiplatformExtension.getSourceSets(): every source set, commonMain included.
        public List<SourceSet> getSourceSets() {
            List<SourceSet> all = new ArrayList<>();
            for (Target target : targets)
                for (Compilation compilation : target.getCompilations().all()) all.addAll(compilation.getKotlinSourceSets());
            all.add(new SourceSet(root, "commonMain"));
            all.add(new SourceSet(root, "commonTest"));
            // A generator's root registered on a source set, as SQLDelight does on commonMain.
            if (generatedCommonRoot) all.add(new SourceSet(new File(root, "build/generated/fake/commonMain/kotlin")));
            return all;
        }
    }
    /** Writes an empty jar: a task-built file dependency. */
    public abstract static class FakeJar extends org.gradle.api.DefaultTask {
        @org.gradle.api.tasks.OutputFile public abstract org.gradle.api.file.RegularFileProperty getOutputFile();
        @org.gradle.api.tasks.TaskAction public void write() throws java.io.IOException {
            java.nio.file.Files.write(getOutputFile().get().getAsFile().toPath(), new byte[0]);
        }
    }
    @Override public void apply(Project project) {
        boolean android = project.hasProperty("fakeAndroidKmp");
        boolean androidKmpLibrary = project.hasProperty("fakeAndroidKmpLibrary");
        boolean androidKmpTests = project.hasProperty("fakeAndroidKmpTests");
        Kotlin kotlin = new Kotlin(project.getProjectDir(), android, androidKmpLibrary, androidKmpTests,
                project.hasProperty("fakeAndroidKmpResources"), project.hasProperty("fakeGeneratedCommonRoot"));
        project.getExtensions().add("kotlin", kotlin);
        for (String name : List.of("jvmMainCompileClasspath", "jvmTestCompileClasspath"))
            project.getConfigurations().create(name).setCanBeResolved(true);
        if (android) for (String name : List.of("androidDebugCompileClasspath",
                "androidDebugUnitTestCompileClasspath", "androidDebugAndroidTestCompileClasspath"))
            project.getConfigurations().create(name).setCanBeResolved(true);
        if (androidKmpLibrary) project.getConfigurations().create("androidCompileClasspath").setCanBeResolved(true);
        if (androidKmpTests) for (String name : List.of("androidHostTestCompileClasspath",
                "androidDeviceTestCompileClasspath", "androidBenchmarkCompileClasspath"))
            project.getConfigurations().create(name).setCanBeResolved(true);
        if (project.hasProperty("fakeOwnOutputs")) {
            // Mirrors KGP 2.4.21: a test compilation's classpath holds its main compilation's
            // outputs as a file dependency built by compileKotlinJvm; a local jar is one too.
            org.gradle.api.tasks.TaskProvider<FakeAndroidPlugin.FakeSourceGenerator> compile = project.getTasks().register(
                    "compileKotlinJvm", FakeAndroidPlugin.FakeSourceGenerator.class,
                    task -> task.getOutputDir().set(project.getLayout().getBuildDirectory().dir("classes/kotlin/jvm/main")));
            project.getDependencies().add("jvmTestCompileClasspath",
                    project.files(compile.flatMap(FakeAndroidPlugin.FakeSourceGenerator::getOutputDir)));
            project.getDependencies().add("jvmTestCompileClasspath", project.files("libs/local.jar"));
            // A jar a task writes under build/ is a real dependency, and its producer must run (#1987).
            org.gradle.api.tasks.TaskProvider<FakeJar> vendor = project.getTasks().register("vendorJar", FakeJar.class,
                    task -> task.getOutputFile().set(project.getLayout().getBuildDirectory().file("vendor/vendor.jar")));
            project.getDependencies().add("jvmTestCompileClasspath",
                    project.files(vendor.flatMap(FakeJar::getOutputFile)));
        }
        project.getGradle().projectsEvaluated(gradle -> {
            for (Target target : kotlin.getTargets())
                for (Compilation compilation : target.getCompilations().all()) compilation.refineCommon();
        });
    }
}
