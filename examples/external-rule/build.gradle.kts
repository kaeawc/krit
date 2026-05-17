plugins {
    id("dev.jasonpearson.krit.custom")
}

// `kritVersion` is forwarded by the integration harness so the example
// resolves the same `krit-rule-api` snapshot that was published to
// mavenLocal during the test setup. When run by hand, fall back to the
// plugin's baked-in default.
val resolvedKritVersion: String? = (findProperty("kritVersion") as String?)?.takeIf { it.isNotBlank() }

kritCustomRules {
    if (resolvedKritVersion != null) {
        ruleApiVersion.set(resolvedKritVersion)
        sdkVersion.set(resolvedKritVersion)
    }
    vendorId.set("example")
}

kotlin {
    jvmToolchain(21)
}
