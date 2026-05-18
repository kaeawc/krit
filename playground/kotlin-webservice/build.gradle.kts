plugins {
    kotlin("jvm") version "2.3.21"
    application
    id("dev.jasonpearson.krit")
}

group = "com.example"
version = "1.0.0"

application {
    mainClass.set("com.example.ApplicationKt")
}

repositories {
    mavenCentral()
}

dependencies {
    implementation("io.ktor:ktor-server-core:3.4.3")
    implementation("io.ktor:ktor-server-netty:3.4.3")
    implementation("io.ktor:ktor-server-content-negotiation:3.4.3")
    implementation("io.ktor:ktor-serialization-kotlinx-json:3.4.3")

    // `dev.jasonpearson.krit.custom` publishes a `krit-rule-bundle` variant
    // here; the host plugin's `kritCustomRules` configuration resolves it.
    kritCustomRules(project(":custom-rules"))
}

krit {
    config = file("krit.yml")
    ignoreFailures = true

    reports {
        plain.required = true
        json.required = true
    }

    advanced {
        // Use the krit binary built from this checkout (`make build` or
        // `go build -o krit ./cmd/krit/` at the repo root) so the demo does
        // not depend on a published release. Drop this block to fall back to
        // the version downloaded from GitHub Releases.
        val localKrit = rootDir.resolve("../../krit")
        if (localKrit.isFile) {
            binary = localKrit
        }
    }
}
