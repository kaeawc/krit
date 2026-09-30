rootProject.name = "krit-types"

include(":krit-rule-api")
project(":krit-rule-api").projectDir = file("../krit-rule-api")

dependencyResolutionManagement {
    repositories {
        mavenCentral()
        maven("https://www.jetbrains.com/intellij-repository/releases") {
            content { includeModule("com.jetbrains.intellij.platform", "core") }
        }
        exclusiveContent {
            forRepository {
                maven("https://redirector.kotlinlang.org/maven/intellij-dependencies")
            }
            filter {
                includeModule("org.jetbrains.kotlin", "kotlin-compiler")
                includeModule("org.jetbrains.kotlin", "kotlin-script-runtime")
                includeModule("org.jetbrains.kotlin", "kotlin-stdlib-jdk7")
                includeModule("org.jetbrains.kotlin", "kotlin-stdlib-jdk8")
                includeModuleByRegex("org.jetbrains.kotlin", ".*-for-ide")
            }
        }
    }
}
