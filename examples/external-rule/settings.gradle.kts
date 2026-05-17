pluginManagement {
    repositories {
        mavenLocal()
        gradlePluginPortal()
        mavenCentral()
    }
    // Build the plugin from source instead of resolving from a Maven
    // repo. Lets the example track local changes without a publish
    // step and keeps `./gradlew :external-rule:kritRuleJar` working
    // straight out of a fresh git clone. CI can still publish to
    // mavenLocal first and resolve there — included builds simply win
    // when both are available.
    includeBuild("../../krit-custom-rule-plugin")
}

dependencyResolutionManagement {
    repositoriesMode.set(RepositoriesMode.FAIL_ON_PROJECT_REPOS)
    repositories {
        mavenLocal()
        mavenCentral()
        // `org.jetbrains.kotlin:kotlin-compiler` (used at compileOnly
        // by the rule API for PSI types) lives in JetBrains' redirector,
        // not Maven Central. Exclusive content keeps everything else
        // resolving from Central.
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

rootProject.name = "external-rule"
