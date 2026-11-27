// Kotlin Multiplatform library with jvm() and an Android target (#1752).
// AGP 9 no longer lets com.android.library share a module with the KMP
// plugin; the Android target comes from com.android.kotlin.multiplatform.library
// (kotlin { android { } }). KSP runs Moshi's codegen on both targets and
// SQLDelight generates the database from commonMain/sqldelight. Gradle only:
// .bazelignore leaves it out (ci/fir-coverage-thresholds.json, gradleOnly).
plugins {
    alias(libs.plugins.kotlin.multiplatform)
    alias(libs.plugins.android.kotlin.multiplatform.library)
    alias(libs.plugins.ksp)
    alias(libs.plugins.sqldelight)
}

kotlin {
    jvm {
        compilerOptions {
            jvmTarget = org.jetbrains.kotlin.gradle.dsl.JvmTarget.JVM_17
        }
    }

    android {
        namespace = "dev.krit.firsample.shared"
        compileSdk = 36
        minSdk = 26
        compilerOptions {
            jvmTarget = org.jetbrains.kotlin.gradle.dsl.JvmTarget.JVM_17
        }
    }

    sourceSets {
        jvmMain.dependencies {
            implementation(libs.moshi)
            implementation(libs.sqldelight.sqlite.driver)
        }
        androidMain.dependencies {
            implementation(libs.moshi)
            implementation(libs.sqldelight.android.driver)
        }
    }
}

dependencies {
    add("kspJvm", libs.moshi.kotlin.codegen)
    add("kspAndroid", libs.moshi.kotlin.codegen)
}

sqldelight {
    databases {
        create("SampleDatabase") {
            packageName.set("dev.krit.firsample.shared.db")
        }
    }
}
