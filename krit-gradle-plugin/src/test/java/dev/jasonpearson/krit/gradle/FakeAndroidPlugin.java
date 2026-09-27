package dev.jasonpearson.krit.gradle;

import java.io.File;
import java.util.List;
import java.util.Set;
import org.gradle.api.Plugin;
import org.gradle.api.Project;
import org.gradle.api.DefaultTask;
import org.gradle.api.file.RegularFile;
import org.gradle.api.provider.Property;
import org.gradle.api.provider.Provider;

/** Minimal Android-shaped plugin for TestKit's plugin-ID and model export tests. */
public class FakeAndroidPlugin implements Plugin<Project> {
    // Mirrors AndroidArtifacts.ArtifactType.R_CLASS_JAR = "r-class-jar" in AGP 8.8.0-alpha09.
    public static final String DEPENDENCY_R_CLASS_JAR_ARTIFACT_TYPE = "r-class-jar";
    public static class CompilerOptions {
        private final Property<String> jvmTarget;
        public CompilerOptions(Project project) {
            jvmTarget = project.getObjects().property(String.class);
            jvmTarget.set("17");
        }
        public Property<String> getJvmTarget() { return jvmTarget; }
    }
    public static class CompileKotlin extends DefaultTask {
        private final CompilerOptions compilerOptions = new CompilerOptions(getProject());
        public CompilerOptions getCompilerOptions() { return compilerOptions; }
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
        public Android(File root) { sources = new Sources(root); }
        public Sources getSourceSets() { return sources; }
    }

    public static class Sdk {
        private final Provider<List<RegularFile>> boot;
        public Sdk(Project project) {
            boot = project.provider(() -> List.of(project.getLayout().getProjectDirectory().file("android.jar")));
        }
        // Mirrors SdkComponents.getBootClasspath() in gradle-api:8.8.0-alpha09.
        public Provider<List<RegularFile>> getBootClasspath() { return boot; }
    }

    public static class Components {
        private final Sdk sdk;
        public Components(Project project) { sdk = new Sdk(project); }
        public Sdk getSdkComponents() { return sdk; }
        // Mirrors AndroidComponents.getPluginVersion().getMajor() in AGP 8.8.0-alpha09.
        public Version getPluginVersion() { return new Version(); }
    }
    public static class Version { public int getMajor() { return 8; } }

    @Override public void apply(Project project) {
        project.getExtensions().add("android", new Android(project.getProjectDir()));
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
        for (String name : List.of("debugCompileClasspath", "releaseCompileClasspath",
                "stagingDebugCompileClasspath",
                "debugUnitTestCompileClasspath", "debugAndroidTestCompileClasspath",
                "stagingDebugUnitTestCompileClasspath", "stagingDebugAndroidTestCompileClasspath")) {
            project.getConfigurations().create(name).setCanBeResolved(true);
        }
        // Mirrors GenerateLibraryRFileTask.CreationAction.getName() and LinkApplicationAndroidResourcesTask.BaseCreationAction.getName() in AGP 8.8.0-alpha09.
        for (String name : List.of("generateDebugRFile", "processDebugResources"))
            project.getTasks().register(name);
        project.getTasks().register("compileDebugKotlin", CompileKotlin.class);
    }
}
