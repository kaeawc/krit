package test;

import java.io.FileInputStream;
import java.io.ObjectInputStream;

class Decoder {
    Object decode(String path) throws Exception {
        return new ObjectInputStream(new FileInputStream(path)).readObject();
    }
}
