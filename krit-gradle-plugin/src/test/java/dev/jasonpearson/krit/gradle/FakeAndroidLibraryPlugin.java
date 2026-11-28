package dev.jasonpearson.krit.gradle;

import org.gradle.api.Project;

/**
 * com.android.library's fake. A class of its own, as in AGP, so a module that applies another
 * Android plugin ID is not also a library to class-based `hasPlugin` checks.
 */
public class FakeAndroidLibraryPlugin extends FakeAndroidPlugin {
    @Override public void apply(Project project) {
        // AGP refuses a second Android plugin on one module; tests that apply several Android IDs
        // to one module (to see each ID's callback) keep the first one's setup.
        if (project.getExtensions().findByName("android") != null) return;
        super.apply(project);
    }
}
