rootProject.name = "krit-types"

dependencyResolutionManagement {
    repositories {
        mavenCentral()
        exclusiveContent {
            forRepository {
                maven("https://redirector.kotlinlang.org/maven/intellij-dependencies")
            }
            filter {
                includeModuleByRegex("org.jetbrains.kotlin", ".*-for-ide")
            }
        }
    }
}
