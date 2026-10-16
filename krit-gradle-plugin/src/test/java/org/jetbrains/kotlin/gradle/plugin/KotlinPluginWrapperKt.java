package org.jetbrains.kotlin.gradle.plugin;

import org.gradle.api.Project;

/**
 * Stand-in for KGP's {@code getKotlinPluginVersion(Project)} extension function, which the
 * krit plugin resolves reflectively through the kotlin extension's class loader. Throws when
 * the project sets {@code noKgpVersion}, to exercise the fallback.
 */
public final class KotlinPluginWrapperKt {
    private KotlinPluginWrapperKt() {}

    public static String getKotlinPluginVersion(Project project) {
        if (project.hasProperty("noKgpVersion")) throw new IllegalStateException("unavailable");
        return "2.2.10";
    }
}
