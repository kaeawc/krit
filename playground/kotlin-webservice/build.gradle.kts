import org.gradle.api.tasks.bundling.Jar

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
}

// `kritRuleJar` (provided by dev.jasonpearson.krit.custom) is the stamped jar
// that contains both the generated `META-INF/services/...KritRule` file and the
// `Krit-SDK-Version` / `Krit-Vendor-Id` manifest attributes that the krit
// daemon reads at load time. The default `jar` task from kotlin("jvm") does
// not include those, so consume the stamped jar explicitly. The
// `evaluationDependsOn` call ensures :custom-rules has registered its tasks
// before we look up `kritRuleJar`.
evaluationDependsOn(":custom-rules")
val customRulesJar = project(":custom-rules").tasks.named("kritRuleJar", Jar::class.java)

krit {
    config.set(file("krit.yml"))
    ignoreFailures.set(true)
    customRules(customRulesJar.flatMap { it.archiveFile })

    // Use the krit binary built from this checkout (`make build` or
    // `go build -o krit ./cmd/krit/` at the repo root) so the demo does not
    // depend on a published release. Drop this line to fall back to the
    // version downloaded from GitHub Releases.
    val localKrit = rootDir.resolve("../../krit")
    if (localKrit.isFile) {
        binary.set(localKrit)
    }

    reports.plain.required.set(true)
    reports.json.required.set(true)
}
