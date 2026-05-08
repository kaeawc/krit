package test

class ProcessBuilderShellArgFixture {
    fun grep(pattern: String) {
        ProcessBuilder("sh", "-c", "grep $pattern /var/log/app.log").start()
        ProcessBuilder(listOf("bash", "-c", "grep " + pattern + " /var/log/app.log")).start()
    }
}
