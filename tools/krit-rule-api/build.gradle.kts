plugins {
    kotlin("jvm") version "2.3.21"
    `java-library`
}

kotlin {
    jvmToolchain(21)
}

java {
    withSourcesJar()
}
