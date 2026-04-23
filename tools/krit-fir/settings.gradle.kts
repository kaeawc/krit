rootProject.name = "krit-fir"
include("compiler-tests")

dependencyResolutionManagement {
    repositories {
        mavenCentral()
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
