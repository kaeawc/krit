// Positive fixture: targetSdk is below the rule's default floor (34).
// The rule fires Fatal because the configured value is no longer
// accepted under modern Play Store compliance policy.
plugins {
    id("com.android.application")
}

android {
    namespace = "com.example.expiredtarget"
    compileSdk = 34

    defaultConfig {
        applicationId = "com.example.expiredtarget"
        minSdk = 24
        targetSdk = 28
    }
}
