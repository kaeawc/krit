plugins {
    id("signal-android-app-conventions")
}

dependencies {
    implementation(project(":core-util"))
    implementation(project(":libsignal-service"))
    implementation(project(":billing"))
    implementation(project(":device-transfer"))
    implementation(project(":contacts"))
    implementation(project(":spinner"))
    implementation(project(":glide-config"))
    implementation(project(":image-editor"))
    implementation(project(":video-trimmer"))
    implementation(project(":qr"))
    implementation(project(":donations"))
    testImplementation(testFixtures(project(":libsignal-service")))
    testImplementation(project(":test-utils"))
}
