package test;

import java.io.File;
import java.io.IOException;
import java.nio.file.Files;

public class TempFileWorldReadable {
    void makeReadable() throws IOException {
        File t = File.createTempFile("secret", ".txt");
        t.setReadable(true, false);
    }

    void makeWritable() throws IOException {
        File t = File.createTempFile("secret", ".txt");
        t.setWritable(true, false);
    }

    void makeExecutable() throws IOException {
        File t = File.createTempFile("secret", ".sh");
        t.setExecutable(true, false);
    }

    void viaFiles() throws IOException {
        File t = Files.createTempFile("secret", ".txt").toFile();
        t.setReadable(true, false);
    }
}
