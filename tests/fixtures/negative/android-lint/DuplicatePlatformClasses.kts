// Negative fixture: all dependencies are ordinary libraries that do not
// duplicate Android platform classes, so DuplicatePlatformClasses stays
// silent. okhttp is the recommended replacement for the platform HTTP stack.
plugins {
    id("com.android.application")
}

dependencies {
    implementation("androidx.core:core-ktx:1.12.0")
    implementation("androidx.appcompat:appcompat:1.6.1")
    implementation("com.squareup.okhttp3:okhttp:4.12.0")
}
