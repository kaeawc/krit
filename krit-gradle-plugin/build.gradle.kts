plugins {
    `java-gradle-plugin`
    `kotlin-dsl`
    `maven-publish`
}

group = "dev.krit"
version = "0.1.0"

repositories {
    mavenCentral()
}

gradlePlugin {
    plugins {
        create("krit") {
            id = "dev.krit"
            implementationClass = "dev.krit.gradle.KritPlugin"
            displayName = "Krit Kotlin Lint"
            description = "Static analysis for Kotlin using tree-sitter"
        }
    }
}

dependencies {
    testImplementation(gradleTestKit())
    testImplementation("org.junit.jupiter:junit-jupiter:5.10.2")
}

tasks.test {
    useJUnitPlatform()
}
