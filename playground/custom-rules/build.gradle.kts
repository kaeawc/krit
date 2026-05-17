plugins {
    id("dev.jasonpearson.krit.custom")
}

kotlin {
    jvmToolchain(21)
}

kritCustomRules {
    vendorId.set("playground")
    defaultSeverity.set("warning")
}
