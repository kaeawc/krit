package test;

import org.apache.http.conn.ssl.AllowAllHostnameVerifier;
import org.apache.http.conn.ssl.SSLSocketFactory;

class AllowAll {
    Object verifier = new AllowAllHostnameVerifier();
    Object defaultVerifier = SSLSocketFactory.ALLOW_ALL_HOSTNAME_VERIFIER;
}
