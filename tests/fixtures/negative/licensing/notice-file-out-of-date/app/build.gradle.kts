plugins {
    id("org.jetbrains.kotlin.jvm")
}

dependencies {
    // Intended future "requires notice" registry match.
    implementation("com.example:attrib-required-lib:1.2.3")
    implementation("org.jetbrains.kotlin:kotlin-stdlib:2.0.0")
}
