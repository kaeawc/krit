package test;

class ProcessBuilderShellArgJavaFixture {
    void grep(String pattern) throws java.io.IOException {
        new ProcessBuilder("sh", "-c", "grep " + pattern + " /var/log/app.log").start();
    }
}
