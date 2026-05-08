package test;

import javax.net.ssl.HostnameVerifier;
import javax.net.ssl.SSLSession;

class ValidatingVerifier implements HostnameVerifier {
    public boolean verify(String hostname, SSLSession session) {
        return hostname.equals(session.getPeerHost());
    }
}
