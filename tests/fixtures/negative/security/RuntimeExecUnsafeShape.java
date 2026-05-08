package test;

class RuntimeExecUnsafeShapeJavaSafeFixture {
    void list(String userPath) throws java.io.IOException {
        Runtime.getRuntime().exec(new String[] {"ls", "-la", userPath});
        Runtime.getRuntime().exec("ls -la");
        new ProcessBuilder("ls", "-la", userPath).start();
    }
}
