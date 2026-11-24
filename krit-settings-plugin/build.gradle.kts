import org.gradle.api.publish.maven.tasks.PublishToMavenRepository

plugins {
    `java-gradle-plugin`
    `kotlin-dsl`
    `maven-publish`
    signing
    id("com.gradle.plugin-publish") version "2.2.1"
}

group = "dev.jasonpearson.krit"
version = (findProperty("kritVersion") as String?)
    ?.takeIf { it.isNotBlank() }
    ?: System.getenv("KRIT_VERSION")?.takeIf { it.isNotBlank() }
    ?: "0.0.0-SNAPSHOT"

val isSnapshot = version.toString().endsWith("-SNAPSHOT")

repositories {
    mavenCentral()
}

// Intentionally no `jvmToolchain(...)` — the composite-built dependency on
// `krit-gradle-plugin` matches whichever JVM Gradle itself runs on, and a
// pinned consumer toolchain produces a no-matching-variant error. The
// bytecode target is pinned to 17 instead (as in krit-gradle-plugin), so the
// published plugin loads in builds whose Gradle runs on JDK 17.
tasks.withType<JavaCompile>().configureEach {
    options.release.set(17)
}

tasks.withType<org.jetbrains.kotlin.gradle.tasks.KotlinCompile>().configureEach {
    compilerOptions.jvmTarget.set(org.jetbrains.kotlin.gradle.dsl.JvmTarget.JVM_17)
}

// com.gradle.plugin-publish adds the sources and javadoc jars Maven Central
// requires, and `publishPlugins` uploads the same publications to the Gradle
// Plugin Portal.
gradlePlugin {
    website = "https://github.com/kaeawc/krit"
    vcsUrl = "https://github.com/kaeawc/krit"
    plugins {
        create("kritSettings") {
            id = "dev.jasonpearson.krit.settings"
            implementationClass = "dev.jasonpearson.krit.settings.KritSettingsPlugin"
            displayName = "Krit Settings Plugin"
            description = "Apply Krit static analysis to every Kotlin / Java / " +
                "Android subproject from a single `settings.gradle.kts` block."
            tags = listOf("kotlin", "android", "lint", "static-analysis", "code-quality")
        }
    }
}

dependencies {
    // Depend on the host project plugin so the settings plugin can apply it by
    // type and read `KritExtension` directly. The version matches this plugin's
    // own (the two ship in lockstep) so the published POM and module metadata
    // are resolvable from Maven Central; locally the composite build declared
    // in `settings.gradle.kts` substitutes it regardless of version.
    implementation("dev.jasonpearson.krit:krit-gradle-plugin:$version")

    testImplementation(gradleTestKit())
    testImplementation("org.junit.jupiter:junit-jupiter:6.1.3")
    testRuntimeOnly("org.junit.platform:junit-platform-launcher")
}

tasks.test {
    useJUnitPlatform()
    // Surface the project-plugin location to TestKit so the functional test's
    // generated consumer build can `includeBuild(...)` the local source.
    systemProperty(
        "krit.gradle.plugin.dir",
        project.layout.projectDirectory.dir("../krit-gradle-plugin").asFile.absolutePath,
    )
}

// Maven Central: `pluginMaven` is the plugin jar (krit-settings-plugin), and
// each `<name>PluginMarkerMaven` publication is the `<id>.gradle.plugin` marker
// that resolves `plugins { id("dev.jasonpearson.krit.settings") }` from
// mavenCentral().
publishing {
    publications.withType<MavenPublication>().configureEach {
        pom {
            if (name.orNull.isNullOrBlank()) name.set("Krit Settings Plugin")
            if (description.orNull.isNullOrBlank()) {
                description.set("Gradle settings plugin that applies the krit static analyzer to every subproject from settings.gradle.kts.")
            }
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
    repositories {
        maven {
            name = "stagingDir"
            url = layout.buildDirectory.dir("staging-deploy").get().asFile.toURI()
        }
        maven {
            name = "centralPortal"
            url = if (isSnapshot) uri("https://central.sonatype.com/repository/maven-snapshots/")
                else uri("https://ossrh-staging-api.central.sonatype.com/service/local/staging/deploy/maven2/")
            credentials {
                username = System.getenv("SONATYPE_USERNAME").orEmpty()
                password = System.getenv("SONATYPE_PASSWORD").orEmpty()
            }
        }
    }
}

val signingKey: String? = System.getenv("SIGNING_KEY")
val signingPassword: String? = System.getenv("SIGNING_PASSWORD")
val hasSigningKey = !signingKey.isNullOrBlank() && !signingPassword.isNullOrBlank()

signing {
    if (hasSigningKey) {
        useInMemoryPgpKeys(signingKey, signingPassword)
        sign(publishing.publications)
    }
}

// plugin-publish registers sign tasks for every publication whenever the
// signing plugin is applied; skip them for local builds without a key (the
// centralPortal guard below still refuses an unsigned release upload).
//
// The task lambdas capture plain local values, not the script's top-level
// properties: this build runs with the configuration cache on, which cannot
// serialize references to the script object.
run {
    val snapshot = isSnapshot
    val canSign = hasSigningKey

    tasks.withType<Sign>().configureEach {
        onlyIf { !snapshot && canSign }
    }

    // The repository is not bound yet when java-gradle-plugin's marker publish
    // tasks are configured, and reading `repository` from a doFirst action is
    // discarded by the configuration cache, so key the guard on the task name
    // (`publish<Publication>To<Repository>Repository`).
    tasks.withType<PublishToMavenRepository>().configureEach {
        val taskName = name
        doFirst {
            if (taskName.endsWith("ToCentralPortalRepository") && !snapshot) {
                val missing = listOf("SIGNING_KEY", "SIGNING_PASSWORD")
                    .filter { System.getenv(it).isNullOrBlank() }
                if (missing.isNotEmpty()) {
                    throw GradleException("Cannot publish to centralPortal without ${missing.joinToString(" and ")}")
                }
            }
        }
    }
}
