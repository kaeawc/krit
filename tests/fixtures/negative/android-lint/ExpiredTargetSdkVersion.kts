// Negative fixture: targetSdk meets the rule's default floor (34) and
// the rule stays silent. Increasing the value (e.g. 35, 36) also stays
// silent.
plugins {
    id("com.android.application")
}

android {
    namespace = "com.example.compliant"
    compileSdk = 34

    defaultConfig {
        applicationId = "com.example.compliant"
        minSdk = 24
        targetSdk = 34
    }
}
