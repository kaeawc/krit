package test;

class ProcessBuilderShellArgJavaSafeFixture {
    void grep(String pattern) throws java.io.IOException {
        new ProcessBuilder("grep", pattern, "/var/log/app.log").start();
        new ProcessBuilder("sh", "-c", "grep fixed /var/log/app.log").start();
    }
}
