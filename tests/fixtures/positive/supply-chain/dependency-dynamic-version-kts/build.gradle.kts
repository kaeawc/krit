plugins {
    id("com.android.application")
}

dependencies {
    implementation("androidx.core:core-ktx:1.+")
    implementation("com.example:wildcard-deep:2.3.+")
    implementation("com.example:range:[1.0,2.0)")
    implementation("com.example:open-range:[1.5,)")
    implementation("com.example:other-latest:latest.snapshot")
    api(group = "com.example", name = "named-arg", version = "1.+")
}
