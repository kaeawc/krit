plugins {
    id("com.android.application") version "9.2.1"
    id("dev.zacsweers.metro") version "1.0.0"
}

android {
    namespace = "com.example.app"
    compileSdk = 36

    defaultConfig {
        applicationId = "com.example.app"
        minSdk = 23
        targetSdk = 36
        versionCode = 1
        versionName = "1.0"
    }

    buildTypes {
        release {
            isMinifyEnabled = false
        }
    }

    compileOptions {
        sourceCompatibility = JavaVersion.VERSION_17
        targetCompatibility = JavaVersion.VERSION_17
    }
}

dependencies {
    implementation("androidx.core:core-ktx:1.18.0")
    implementation("androidx.appcompat:appcompat:1.7.1")
    implementation("com.google.android.material:material:1.13.0")
    implementation("androidx.recyclerview:recyclerview:1.4.0")
    implementation("androidx.preference:preference-ktx:1.2.1")
    implementation("androidx.room:room-runtime:2.8.4")
    implementation("androidx.work:work-runtime-ktx:2.11.2")
    implementation("com.slack.circuit:circuit-foundation:0.33.1")
    implementation("com.jakewharton.timber:timber:5.0.1")
    implementation("com.squareup.moshi:moshi:1.15.2")
    implementation("com.squareup.moshi:moshi-kotlin:1.15.2")
    implementation("com.squareup.okhttp3:okhttp:5.3.2")
    implementation("com.squareup.retrofit2:retrofit:3.0.0")
    implementation("dev.zacsweers.metro:runtime:1.0.0")
    implementation("io.coil-kt.coil3:coil:3.4.0")
}
