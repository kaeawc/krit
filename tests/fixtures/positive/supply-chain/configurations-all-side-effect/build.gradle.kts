plugins {
    kotlin("jvm") version "1.9.24"
}

configurations.all {
    resolutionStrategy.force("com.squareup.okhttp3:okhttp:4.12.0")
}
