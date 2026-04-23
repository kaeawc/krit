plugins {
    id("kotlin-multiplatform")
    id("maven-publish")
}

dependencies {
    api(projects.backstack)
    api(projects.circuitRuntime)
    api(projects.circuitRetained)
    api(projects.circuitOverlay)
    implementation(projects.internalTestUtils)
    api(projects.circuitTest)
}
