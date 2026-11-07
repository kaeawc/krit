plugins {
    id("org.jetbrains.kotlin.jvm") version "2.4.20"
}

kotlin {
    compilerOptions {
        freeCompilerArgs.add("-Xwhen-guards")
        freeCompilerArgs.addAll("-Xcontext-parameters", "-Xexplicit-backing-fields")
        freeCompilerArgs.addAll(
            "-opt-in=kotlin.RequiresOptIn",
            "-Xnon-local-break-continue",
        )
        freeCompilerArgs += listOf("-Xmulti-dollar-interpolation", "-Xsam-conversions=class")
    }
}
