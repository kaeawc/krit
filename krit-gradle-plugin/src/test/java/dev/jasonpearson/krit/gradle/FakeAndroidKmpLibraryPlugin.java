package dev.jasonpearson.krit.gradle;

import java.util.List;
import org.gradle.api.Plugin;
import org.gradle.api.Project;
import org.gradle.api.file.Directory;
import org.gradle.api.provider.ListProperty;
import org.gradle.api.tasks.TaskProvider;

/**
 * com.android.kotlin.multiplatform.library-shaped fixture (AGP 9.5.3): `androidComponents` and no
 * top-level `android` extension. Its single variant, androidMain, is created only after the
 * project's afterEvaluate callbacks ran; with -PfakeGeneratedSources its Kotlin roots include a
 * generated one, like the root kspAndroidMain registers.
 */
public class FakeAndroidKmpLibraryPlugin implements Plugin<Project> {
    @Override public void apply(Project project) {
        FakeAndroidPlugin.Components components = new FakeAndroidPlugin.Components(project);
        project.getExtensions().add("androidComponents", components);
        project.getGradle().projectsEvaluated(gradle -> {
            Directory androidMain = project.getLayout().getProjectDirectory().dir("src/androidMain/kotlin");
            ListProperty<Directory> kotlin = project.getObjects().listProperty(Directory.class);
            kotlin.add(androidMain);
            if (project.hasProperty("fakeGeneratedSources")) {
                TaskProvider<FakeAndroidPlugin.FakeSourceGenerator> generator = project.getTasks().register(
                        "kspAndroidMain", FakeAndroidPlugin.FakeSourceGenerator.class,
                        task -> task.getOutputDir().set(project.getLayout().getBuildDirectory().dir("generated/ksp/android/androidMain/kotlin")));
                kotlin.add(generator.flatMap(FakeAndroidPlugin.FakeSourceGenerator::getOutputDir));
            }
            components.runVariantActions(new FakeAndroidPlugin.Component("androidMain",
                    new FakeAndroidPlugin.VariantSources(null, new FakeAndroidPlugin.Directories(kotlin)), null));
        });
    }
}
