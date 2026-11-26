plugins {
    id("com.android.application")
}

android {
    namespace = "com.example.usesdksuppress"
    compileSdk = 36

    defaultConfig {
        applicationId = "com.example.usesdksuppress"
        minSdk = 24
        targetSdk = 35
    }
}

dependencies {
    implementation("androidx.annotation:annotation:1.9.1")
}

dependencies {
    testImplementation("junit:junit:4.13.2")
}

dependencies {
    implementation("junit:junit:4.13.2")
}
