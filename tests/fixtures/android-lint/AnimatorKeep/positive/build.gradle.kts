plugins {
    id("com.android.application") version "8.5.0"
}

android {
    namespace = "com.example"
    buildTypes {
        release {
            isMinifyEnabled = true
            isShrinkResources = true
        }
    }
}
