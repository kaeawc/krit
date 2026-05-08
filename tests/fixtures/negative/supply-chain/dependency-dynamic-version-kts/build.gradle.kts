plugins {
    id("com.android.application")
}

// Local DSL helper that shadows the Kotlin Gradle DSL `implementation` call.
// A dynamic-looking string passed to it must NOT be flagged.
fun implementation(coordinate: String): String = coordinate

val unused = implementation("com.example:local-shadow:1.+")

dependencies {
    implementation("androidx.core:core-ktx:1.12.0")
    implementation("com.example:lib:2.3.4")
    api(group = "com.example", name = "named-arg-pinned", version = "1.2.3")
    // implementation("com.example:commented-dynamic:1.+")
    // implementation(group = "com.example", name = "commented-named-arg", version = "1.+")
}
