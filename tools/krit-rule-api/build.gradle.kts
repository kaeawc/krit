plugins {
    kotlin("jvm") version "2.3.21"
    `java-library`
    id("com.vanniktech.maven.publish") version "0.36.0"
}

// Coordinates and POM metadata are sourced from gradle.properties so
// the release workflow only has to override VERSION_NAME via -P.
// `mavenPublishing.coordinates(...)` below sets group / version on the
// publication directly — don't set project.version here, doing so
// finalizes a property the vanniktech plugin needs to write later.
// Pattern mirrors ~/kaeawc/auto-mobile/android.

kotlin {
    jvmToolchain(21)
}

java {
    withSourcesJar()
}

mavenPublishing {
    coordinates(
        property("GROUP").toString(),
        property("POM_ARTIFACT_ID").toString(),
        property("VERSION_NAME").toString(),
    )

    publishToMavenCentral(automaticRelease = true)
    signAllPublications()

    // POM name, description, url, inceptionYear, license, developer,
    // and scm blocks are auto-populated by the plugin from the
    // `POM_*` keys in gradle.properties. Only issueManagement isn't
    // covered by the plugin's auto-population, so set it explicitly.
    pom {
        issueManagement {
            system.set("GitHub")
            url.set(property("POM_ISSUES_URL").toString())
        }
    }
}
