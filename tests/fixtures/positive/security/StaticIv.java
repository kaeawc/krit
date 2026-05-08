package test;

import javax.crypto.spec.GCMParameterSpec;
import javax.crypto.spec.IvParameterSpec;

class Crypto {
    void params() {
        new IvParameterSpec(new byte[] {0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0});
        new GCMParameterSpec(128, "000000000000".getBytes());
    }
}
