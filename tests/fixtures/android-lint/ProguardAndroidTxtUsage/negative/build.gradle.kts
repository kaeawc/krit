plugins {
    id("com.android.application") version "8.5.0"
}

android {
    buildTypes {
        release {
            isMinifyEnabled = true
            isShrinkResources = true
            // getDefaultProguardFile("proguard-android.txt") is the unoptimized file.
            proguardFiles(getDefaultProguardFile("proguard-android-optimize.txt"), "proguard-android.txt")
        }
    }
}
