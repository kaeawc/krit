// Compiler-test source stub; never packaged in the production artifact.
package android.security.keystore;

public abstract class KeyProperties {
    KeyProperties() {
    }

    public static final int PURPOSE_ENCRYPT = 1;
    public static final int PURPOSE_DECRYPT = 2;
    public static final int PURPOSE_SIGN = 4;
    public static final int PURPOSE_VERIFY = 8;
    public static final String KEY_ALGORITHM_RSA = "RSA";
    public static final String KEY_ALGORITHM_EC = "EC";
    public static final String KEY_ALGORITHM_AES = "AES";
    public static final String KEY_ALGORITHM_HMAC_SHA256 = "HmacSHA256";
    public static final String BLOCK_MODE_ECB = "ECB";
    public static final String BLOCK_MODE_GCM = "GCM";
    public static final String ENCRYPTION_PADDING_NONE = "NoPadding";
}
