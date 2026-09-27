package dev.jasonpearson.krit.gradle;

import org.gradle.api.Plugin;
import org.gradle.api.Project;

/** Code-generation task fixture for opt-in task graph checks. */
public class FakeKspPlugin implements Plugin<Project> {
    @Override public void apply(Project project) {
        // Mirrors KspGradleSubplugin.applyToCompilation in KSP 2.2.20-2.0.4: compileDebugKotlin -> kspDebugKotlin.
        project.getTasks().register("kspDebugKotlin");
        project.getTasks().register("kspDebugKotlinAndroid");
        project.getTasks().register("kspKotlin");
        project.getTasks().register("kspKotlinJvm");
    }
}
