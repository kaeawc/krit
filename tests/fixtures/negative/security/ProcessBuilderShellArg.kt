package test

class ProcessBuilderShellArgSafeFixture {
    fun grep(pattern: String) {
        ProcessBuilder("grep", pattern, "/var/log/app.log").start()
        ProcessBuilder("sh", "-c", "grep fixed /var/log/app.log").start()
        ProcessBuilder("sh", "-lc", "grep $pattern /var/log/app.log").start()
    }
}
