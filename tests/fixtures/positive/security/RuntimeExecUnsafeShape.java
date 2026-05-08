package test;

class RuntimeExecUnsafeShapeJavaFixture {
    void list(String userPath) throws java.io.IOException {
        Runtime.getRuntime().exec("ls -la " + userPath);
    }
}
