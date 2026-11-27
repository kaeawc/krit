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
        public SourceSet(File root, String name) { kotlin = new Dirs(new File(root, "src/" + name + "/kotlin")); }
        public Dirs getKotlin() { return kotlin; }
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
    public static class Compilations {
        private final Map<String, Compilation> compilations;
        public Compilations(File root, String target, boolean android, boolean androidKmpLibrary) {
            // Mirrors KotlinAndroidTarget.getCompilations() and AndroidProjectHandlerKt.getVariantName() in KGP 2.2.21;
            // the Android-KMP library target (AGP 9.5.3) has one `main` compilation while its host and
            // device tests are not enabled.
            compilations = androidKmpLibrary
                    ? Map.of("main", new Compilation(root, target, "main", "androidCompileClasspath", "compileAndroidMain"))
                    : android
                    ? Map.of("debug", new Compilation(root, target, "debug"),
                        "debugUnitTest", new Compilation(root, target, "debugUnitTest"),
                        "debugAndroidTest", new Compilation(root, target, "debugAndroidTest"))
                    : Map.of("main", new Compilation(root, target, "main"),
                        "test", new Compilation(root, target, "test"));
        }
        public Compilation findByName(String name) { return compilations.get(name); }
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
        public Target(File root, String name, String platform, boolean androidKmpLibrary) {
            this.name = name;
            platformType = new Platform(platform);
            compilations = new Compilations(root, name, platform.equals("androidJvm"), androidKmpLibrary);
        }
        public String getName() { return name; }
        public Platform getPlatformType() { return platformType; }
        public Compilations getCompilations() { return compilations; }
    }
    public static class Kotlin {
        private final List<Target> targets;
        private final File root;
        public Kotlin(File root, boolean android, boolean androidKmpLibrary) {
            this.root = root;
            targets = android || androidKmpLibrary
                    ? List.of(new Target(root, "android", "androidJvm", androidKmpLibrary), new Target(root, "jvm", "jvm", false))
                    : List.of(new Target(root, "jvm", "jvm", false), new Target(root, "js", "js", false));
        }
        public List<Target> getTargets() { return targets; }
        // Mirrors KotlinMultiplatformExtension.getSourceSets(): every source set, commonMain included.
        public List<SourceSet> getSourceSets() {
            List<SourceSet> all = new ArrayList<>();
            for (Target target : targets)
                for (Compilation compilation : target.getCompilations().all()) all.addAll(compilation.getKotlinSourceSets());
            all.add(new SourceSet(root, "commonMain"));
            all.add(new SourceSet(root, "commonTest"));
            return all;
        }
    }
    @Override public void apply(Project project) {
        boolean android = project.hasProperty("fakeAndroidKmp");
        boolean androidKmpLibrary = project.hasProperty("fakeAndroidKmpLibrary");
        Kotlin kotlin = new Kotlin(project.getProjectDir(), android, androidKmpLibrary);
        project.getExtensions().add("kotlin", kotlin);
        for (String name : List.of("jvmMainCompileClasspath", "jvmTestCompileClasspath"))
            project.getConfigurations().create(name).setCanBeResolved(true);
        if (android) for (String name : List.of("androidDebugCompileClasspath",
                "androidDebugUnitTestCompileClasspath", "androidDebugAndroidTestCompileClasspath"))
            project.getConfigurations().create(name).setCanBeResolved(true);
        if (androidKmpLibrary) project.getConfigurations().create("androidCompileClasspath").setCanBeResolved(true);
        project.getGradle().projectsEvaluated(gradle -> {
            for (Target target : kotlin.getTargets())
                for (Compilation compilation : target.getCompilations().all()) compilation.refineCommon();
        });
    }
}
