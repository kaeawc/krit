plugins {
    id("org.jetbrains.kotlin.jvm")
}

dependencies {
    // Intended future embedded-registry match: GPL-3.0
    implementation("fixture.registry:gpl3-only-lib:1.0.0")
    implementation("org.jetbrains.kotlin:kotlin-stdlib:2.0.0")
}
