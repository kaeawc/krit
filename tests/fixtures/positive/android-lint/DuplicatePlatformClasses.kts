// Positive fixture: commons-logging duplicates android.util.Log classes that
// already ship in the Android platform. DuplicatePlatformClasses fires Fatal
// on the offending dependency line.
plugins {
    id("com.android.application")
}

dependencies {
    implementation("androidx.core:core-ktx:1.12.0")
    implementation("commons-logging:commons-logging:1.2")
    implementation("org.apache.httpcomponents:httpclient:4.5.13")
}
