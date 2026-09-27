package dev.jasonpearson.krit.gradle;

import java.io.File;
import java.util.List;
import java.util.Set;
import org.gradle.api.Plugin;
import org.gradle.api.Project;
import org.gradle.api.file.RegularFile;
import org.gradle.api.provider.Provider;

/** Minimal Android-shaped plugin for TestKit's plugin-ID and model export tests. */
public class FakeAndroidPlugin implements Plugin<Project> {
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
        public Source getByName(String name) {
            if (!Set.of("main", "debug", "release").contains(name))
                throw new IllegalArgumentException(name);
            return new Source(root, name);
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
        public Provider<List<RegularFile>> getBootClasspath() { return boot; }
    }

    public static class Components {
        private final Sdk sdk;
        public Components(Project project) { sdk = new Sdk(project); }
        public Sdk getSdkComponents() { return sdk; }
    }

    @Override public void apply(Project project) {
        project.getExtensions().add("android", new Android(project.getProjectDir()));
        if (!project.hasProperty("omitComponents"))
            project.getExtensions().add("androidComponents", new Components(project));
        for (String name : List.of("debugCompileClasspath", "releaseCompileClasspath",
                "debugUnitTestCompileClasspath")) {
            project.getConfigurations().create(name).setCanBeResolved(true);
        }
    }
}
