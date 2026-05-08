plugins {
    id("org.jetbrains.kotlin.jvm")
}

dependencies {
    // Intended future embedded-registry match: Apache-2.0
    implementation("fixture.registry:apache-friendly-lib:1.0.0")
    implementation("org.jetbrains.kotlin:kotlin-stdlib:2.0.0")
}
