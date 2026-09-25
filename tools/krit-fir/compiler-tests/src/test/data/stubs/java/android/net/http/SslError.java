// Compiler-test source stub; never packaged in the production artifact.
package android.net.http;

public class SslError {
    public static final int SSL_NOTYETVALID = 0;

    public static final int SSL_EXPIRED = 1;

    public static final int SSL_IDMISMATCH = 2;

    public static final int SSL_UNTRUSTED = 3;

    public static final int SSL_DATE_INVALID = 4;

    public static final int SSL_INVALID = 5;

    SslError() {
    }

    public int getPrimaryError() {
        throw new RuntimeException("Stub!");
    }

    public String getUrl() {
        throw new RuntimeException("Stub!");
    }

    public boolean hasError(int error) {
        throw new RuntimeException("Stub!");
    }
}
