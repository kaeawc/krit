plugins {
    id("com.android.application")
}

android {
    namespace = "com.example.wronggradlemethod"
    compileSdk = 36

    buildTypes {
        getByName("debug") {
            dependencies {
                implementation("androidx.annotation:annotation:1.9.1")
            }
        }
        release {
            dependencies {
                implementation("androidx.annotation:annotation:1.9.1")
            }
        }
    }

    productFlavors {
        create("free") {
            dependencies {
                implementation("androidx.annotation:annotation:1.9.1")
            }
        }
    }

    defaultConfig {
        applicationId = "com.example.wronggradlemethod"
        minSdk = 24
        targetSdk = 35
    }
    flavorDimensions += "tier"
    productFlavors.all {
        dimension = "tier"
    }
}
