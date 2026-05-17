rootProject.name = "krit-rule-api"

dependencyResolutionManagement {
    repositories {
        mavenCentral()
        // kotlin-compiler-embeddable relocates `com.intellij.*` and
        // would NoClassDefFoundError at runtime against the daemon's
        // non-embeddable kotlin-compiler. Pull the same artifact the
        // daemon links — only available from JetBrains' redirector.
        exclusiveContent {
            forRepository {
                maven("https://redirector.kotlinlang.org/maven/intellij-dependencies")
            }
            filter {
                includeModule("org.jetbrains.kotlin", "kotlin-compiler")
            }
        }
    }
}
