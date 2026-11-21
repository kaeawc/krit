package stubs

fun lintSmoke() {
    com.android.tools.lint.checks.infrastructure.lint().files("source").run().expectClean()
    com.android.tools.lint.checks.infrastructure.lint().run().expectErrorCount(0)
}
