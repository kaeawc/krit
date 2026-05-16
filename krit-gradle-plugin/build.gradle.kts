plugins {
    `java-gradle-plugin`
    `kotlin-dsl`
    `maven-publish`
}

group = "dev.jasonpearson.krit"
version = "0.1.0"

repositories {
    mavenCentral()
}

gradlePlugin {
    plugins {
        create("krit") {
            id = "dev.jasonpearson.krit"
            implementationClass = "dev.jasonpearson.krit.gradle.KritPlugin"
            displayName = "Krit Kotlin Lint"
            description = "Static analysis for Kotlin using tree-sitter"
        }
    }
}

dependencies {
    testImplementation(gradleTestKit())
    testImplementation("org.junit.jupiter:junit-jupiter:6.0.3")
    testRuntimeOnly("org.junit.platform:junit-platform-launcher")
}

tasks.test {
    useJUnitPlatform()
}
