plugins {
    kotlin("jvm") version "2.1.0"
}

repositories {
    maven("https://example.com/// TODO not really a comment")
}

dependencies {
    // Release dependency graph is intentionally finalized here.
    implementation("com.squareup.retrofit2:retrofit:2.11.0")
}
