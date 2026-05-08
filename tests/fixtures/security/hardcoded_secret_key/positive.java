package test;

import java.util.Base64;
import javax.crypto.spec.SecretKeySpec;

class HardcodedSecretKeys {
    void keys() {
        new SecretKeySpec(new byte[] {1, 2, 3, 4}, "AES");
        new SecretKeySpec("p@ssw0rd12345678".getBytes(), "AES");
        new SecretKeySpec(Base64.getDecoder().decode("c2VjcmV0MTIzNDU2Nzg="), "AES");
    }
}
