package test;

import java.io.File;
import java.io.IOException;

public class TempFileWorldReadable {
    void ownerOnlyExplicit() throws IOException {
        File t = File.createTempFile("secret", ".txt");
        t.setReadable(true, true);
    }

    void ownerOnlyDefault() throws IOException {
        File t = File.createTempFile("secret", ".txt");
        t.setReadable(true);
    }

    void notFromCreateTempFile() {
        File t = new File("/tmp/known.txt");
        t.setReadable(true, false);
    }

    void otherFile() {
        File other = new File("/tmp/known.txt");
        other.setReadable(true, false);
    }
}
