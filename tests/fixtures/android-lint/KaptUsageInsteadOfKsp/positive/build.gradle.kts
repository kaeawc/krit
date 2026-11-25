plugins {
    id("com.android.application") version "8.5.0"
    kotlin("kapt")
}

dependencies {
    kapt("androidx.room:room-compiler:2.6.1")
    kapt("com.github.bumptech.glide:compiler:4.16.0")
}
