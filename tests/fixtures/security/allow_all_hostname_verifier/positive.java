package test;

import javax.net.ssl.HostnameVerifier;
import javax.net.ssl.SSLSession;

class AllowAll implements HostnameVerifier {
    public boolean verify(String hostname, SSLSession session) {
        return true;
    }
}
