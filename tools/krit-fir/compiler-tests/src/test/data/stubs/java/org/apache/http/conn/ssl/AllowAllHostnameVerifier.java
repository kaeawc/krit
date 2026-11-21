package org.apache.http.conn.ssl;

import javax.net.ssl.HostnameVerifier;
import javax.net.ssl.SSLSession;

public class AllowAllHostnameVerifier implements HostnameVerifier {
    public boolean verify(String hostname, SSLSession session) {
        throw new RuntimeException("Stub!");
    }
}
