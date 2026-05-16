plugins {
    `java-gradle-plugin`
    `kotlin-dsl`
    `maven-publish`
}

group = "dev.jasonpearson.krit"
version = (findProperty("kritVersion") as String?)
    ?.takeIf { it.isNotBlank() }
    ?: System.getenv("KRIT_VERSION")?.takeIf { it.isNotBlank() }
    ?: "0.0.0-SNAPSHOT"

// Default krit-rule-api version baked into the plugin. Consumers can override
// via the `kritCustomRules { ruleApiVersion.set(...) }` DSL.
val defaultRuleApiVersion: String = (findProperty("kritRuleApiVersion") as String?)
    ?.takeIf { it.isNotBlank() }
    ?: version.toString()

repositories {
    mavenCentral()
}

kotlin {
    jvmToolchain(21)
}

gradlePlugin {
    plugins {
        create("kritCustomRule") {
            id = "dev.jasonpearson.krit.custom"
            implementationClass = "dev.jasonpearson.krit.custom.KritCustomRulePlugin"
            displayName = "Krit Custom Rule Authoring"
            description = "Scaffolds Kotlin compile classpath, service registration, " +
                "and a jar task for authoring custom Krit rules."
        }
    }
}

// Bake defaults into a generated source file so they're observable at runtime
// without round-tripping through `findProperty()` on the consumer project.
val generateBuildConfig = tasks.register("generateKritCustomRulePluginBuildConfig") {
    val outputDir = layout.buildDirectory.dir("generated/source/buildConfig/main")
    val pluginVersion = version.toString()
    val ruleApiVersion = defaultRuleApiVersion
    inputs.property("pluginVersion", pluginVersion)
    inputs.property("ruleApiVersion", ruleApiVersion)
    outputs.dir(outputDir)
    doLast {
        val pkgDir = outputDir.get().asFile.resolve("dev/jasonpearson/krit/custom")
        pkgDir.mkdirs()
        pkgDir.resolve("BuildConfig.kt").writeText(
            """
            |package dev.jasonpearson.krit.custom
            |
            |internal object BuildConfig {
            |    const val PLUGIN_VERSION: String = "$pluginVersion"
            |    const val DEFAULT_RULE_API_VERSION: String = "$ruleApiVersion"
            |}
            |
            """.trimMargin()
        )
    }
}

sourceSets {
    named("main") {
        java.srcDir(generateBuildConfig.map { layout.buildDirectory.dir("generated/source/buildConfig/main") })
    }
}

dependencies {
    // Bringing the Kotlin Gradle Plugin onto our runtime classpath lets the
    // plugin auto-apply `kotlin("jvm")` without forcing the consumer to
    // declare it themselves.
    implementation("org.jetbrains.kotlin:kotlin-gradle-plugin:2.3.21")

    testImplementation(gradleTestKit())
    testImplementation("org.junit.jupiter:junit-jupiter:6.0.3")
    testRuntimeOnly("org.junit.platform:junit-platform-launcher")
}

tasks.test {
    useJUnitPlatform()
}

publishing {
    publications {
        // `java-gradle-plugin` registers a "pluginMaven" publication and one
        // `<pluginId>PluginMarkerMaven` per declared plugin. We just attach
        // POM metadata to whichever publications Gradle produces.
        withType<MavenPublication>().configureEach {
            pom {
                url.set("https://github.com/kaeawc/krit")
                inceptionYear.set("2026")
                licenses {
                    license {
                        name.set("MIT License")
                        url.set("https://opensource.org/licenses/MIT")
                        distribution.set("repo")
                    }
                }
                developers {
                    developer {
                        id.set("kaeawc")
                        name.set("Jason Pearson")
                        email.set("jason.d.pearson@gmail.com")
                        url.set("https://github.com/kaeawc")
                    }
                }
                scm {
                    connection.set("scm:git:https://github.com/kaeawc/krit.git")
                    developerConnection.set("scm:git:ssh://git@github.com/kaeawc/krit.git")
                    url.set("https://github.com/kaeawc/krit")
                }
                issueManagement {
                    system.set("GitHub")
                    url.set("https://github.com/kaeawc/krit/issues")
                }
            }
        }
    }
    repositories {
        maven {
            name = "stagingDir"
            url = layout.buildDirectory.dir("staging-deploy").get().asFile.toURI()
        }
    }
}
