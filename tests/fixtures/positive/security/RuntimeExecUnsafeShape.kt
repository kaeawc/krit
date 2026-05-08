package test

class RuntimeExecUnsafeShapeFixture {
    fun list(userPath: String) {
        Runtime.getRuntime().exec("ls -la $userPath")
        Runtime.getRuntime().exec("cat " + userPath)
    }
}
