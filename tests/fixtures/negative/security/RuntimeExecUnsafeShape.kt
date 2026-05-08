package test

class RuntimeExecUnsafeShapeSafeFixture {
    fun list(userPath: String) {
        Runtime.getRuntime().exec(arrayOf("ls", "-la", userPath))
        Runtime.getRuntime().exec("ls -la")
        ProcessBuilder("ls", "-la", userPath).start()
    }
}
