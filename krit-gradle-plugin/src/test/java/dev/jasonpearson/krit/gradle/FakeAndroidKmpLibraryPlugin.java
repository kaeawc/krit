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
    /** Mirrors KotlinMultiplatformAndroidVariant: no getAndroidTest(), test components in getDeviceTests(). */
    public static class KmpVariant {
        private final FakeAndroidPlugin.VariantSources sources;
        private final java.util.Map<String, FakeAndroidPlugin.Component> deviceTests;
        KmpVariant(ListProperty<Directory> kotlin, java.util.Map<String, FakeAndroidPlugin.Component> deviceTests) {
            sources = new FakeAndroidPlugin.VariantSources(null, new FakeAndroidPlugin.Directories(kotlin));
            this.deviceTests = deviceTests;
        }
        public String getName() { return "androidMain"; }
        public FakeAndroidPlugin.VariantSources getSources() { return sources; }
        public java.util.Map<String, FakeAndroidPlugin.Component> getDeviceTests() { return deviceTests; }
    }
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
            if (project.hasProperty("fakeAndroidKmpTests")) {
                // Mirrors AGP 9.5.3's KmpVariantImpl: the device test is reachable only through
                // getDeviceTests() (HasDeviceTests), here with a generated root like kspAndroidDeviceTest's.
                TaskProvider<FakeAndroidPlugin.FakeSourceGenerator> deviceKsp = project.getTasks().register(
                        "kspAndroidDeviceTest", FakeAndroidPlugin.FakeSourceGenerator.class,
                        task -> task.getOutputDir().set(project.getLayout().getBuildDirectory().dir("generated/ksp/android/androidDeviceTest/kotlin")));
                ListProperty<Directory> deviceKotlin = project.getObjects().listProperty(Directory.class);
                deviceKotlin.add(deviceKsp.flatMap(FakeAndroidPlugin.FakeSourceGenerator::getOutputDir));
                FakeAndroidPlugin.Component device = new FakeAndroidPlugin.Component("androidDeviceTest",
                        new FakeAndroidPlugin.VariantSources(null, new FakeAndroidPlugin.Directories(deviceKotlin)), null);
                components.runVariantActions(new KmpVariant(kotlin, java.util.Map.of("AndroidTest", device)));
                return;
            }
            components.runVariantActions(new FakeAndroidPlugin.Component("androidMain",
                    new FakeAndroidPlugin.VariantSources(null, new FakeAndroidPlugin.Directories(kotlin)), null));
        });
    }
}
