import org.jetbrains.intellij.platform.gradle.extensions.intellijPlatform

plugins {
    kotlin("jvm") version "2.3.21"
    id("org.jetbrains.intellij.platform") version "2.16.0"
}

group = "dev.krit"
version = "0.1.0-SNAPSHOT"

repositories {
    mavenCentral()
    intellijPlatform { defaultRepositories() }
}

kotlin {
    jvmToolchain(21)
}

dependencies {
    implementation("com.google.code.gson:gson:2.13.2")

    testImplementation(kotlin("test-junit5"))
    testImplementation("org.junit.jupiter:junit-jupiter-api:6.0.3")
    testRuntimeOnly("org.junit.jupiter:junit-jupiter-engine:6.0.3")
    testRuntimeOnly("org.junit.platform:junit-platform-launcher")

    intellijPlatform {
        intellijIdea("2025.3")
        bundledPlugin("com.intellij.java")
        bundledPlugin("org.jetbrains.kotlin")
        pluginVerifier()
    }
}

intellijPlatform {
    pluginConfiguration {
        id.set("dev.jasonpearson.krit")
        name.set("Krit")
        version.set(project.version.toString())
        description.set("Native IntelliJ integration for Krit diagnostics.")

        ideaVersion {
            sinceBuild.set("253")
            untilBuild.set("261.*")
        }

        vendor {
            name.set("Krit")
            url.set("https://github.com/kaeawc/krit")
        }
    }

    pluginVerification { ides { recommended() } }
}

tasks.test {
    useJUnitPlatform()
}
