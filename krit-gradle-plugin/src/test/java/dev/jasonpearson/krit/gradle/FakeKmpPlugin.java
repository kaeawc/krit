package dev.jasonpearson.krit.gradle;

import java.io.File;
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
        private final String target;
        private final String name;
        private final String config;
        private final List<SourceSet> sourceSets;
        public Compilation(File root, String target, String name) {
            this.target = target;
            this.name = name;
            config = target + name.substring(0, 1).toUpperCase() + name.substring(1) + "CompileClasspath";
            sourceSets = List.of(new SourceSet(root, "common" + name.substring(0, 1).toUpperCase() + name.substring(1)),
                    new SourceSet(root, target + name.substring(0, 1).toUpperCase() + name.substring(1)));
        }
        public String getName() { return name; }
        // Mirrors KotlinCompilationImpl.getCompileKotlinTaskName() and DefaultKotlinCompilationTaskNamesContainerFactory.create() in KGP 2.2.21.
        public String getCompileKotlinTaskName() {
            return "compile" + (name.equals("main") ? "" : name.substring(0, 1).toUpperCase() + name.substring(1))
                    + "Kotlin" + target.substring(0, 1).toUpperCase() + target.substring(1);
        }
        public String getCompileDependencyConfigurationName() { return config; }
        public List<SourceSet> getKotlinSourceSets() { return sourceSets; }
    }
    public static class Compilations {
        private final Map<String, Compilation> compilations;
        public Compilations(File root, String target, boolean android) {
            // Mirrors KotlinAndroidTarget.getCompilations() and AndroidProjectHandlerKt.getVariantName() in KGP 2.2.21.
            compilations = android
                    ? Map.of("debug", new Compilation(root, target, "debug"),
                        "debugUnitTest", new Compilation(root, target, "debugUnitTest"),
                        "debugAndroidTest", new Compilation(root, target, "debugAndroidTest"))
                    : Map.of("main", new Compilation(root, target, "main"),
                        "test", new Compilation(root, target, "test"));
        }
        public Compilation findByName(String name) { return compilations.get(name); }
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
        public Target(File root, String name, String platform) {
            this.name = name;
            platformType = new Platform(platform);
            compilations = new Compilations(root, name, platform.equals("androidJvm"));
        }
        public String getName() { return name; }
        public Platform getPlatformType() { return platformType; }
        public Compilations getCompilations() { return compilations; }
    }
    public static class Kotlin {
        private final List<Target> targets;
        public Kotlin(File root, boolean android) {
            targets = android
                    ? List.of(new Target(root, "android", "androidJvm"), new Target(root, "jvm", "jvm"))
                    : List.of(new Target(root, "jvm", "jvm"), new Target(root, "js", "js"));
        }
        public List<Target> getTargets() { return targets; }
    }
    @Override public void apply(Project project) {
        boolean android = project.hasProperty("fakeAndroidKmp");
        project.getExtensions().add("kotlin", new Kotlin(project.getProjectDir(), android));
        for (String name : List.of("jvmMainCompileClasspath", "jvmTestCompileClasspath"))
            project.getConfigurations().create(name).setCanBeResolved(true);
        if (android) for (String name : List.of("androidDebugCompileClasspath",
                "androidDebugUnitTestCompileClasspath", "androidDebugAndroidTestCompileClasspath"))
            project.getConfigurations().create(name).setCanBeResolved(true);
    }
}
