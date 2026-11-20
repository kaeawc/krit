package org.apache.http.conn.ssl;

import java.security.KeyStore;
import javax.net.ssl.HostnameVerifier;

public class SSLSocketFactory {
    public static final HostnameVerifier ALLOW_ALL_HOSTNAME_VERIFIER = null;

    public SSLSocketFactory(KeyStore keyStore) { throw new RuntimeException("Stub!"); }
}
