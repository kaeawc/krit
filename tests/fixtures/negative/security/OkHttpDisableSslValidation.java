package test;

import okhttp3.OkHttpClient;
import javax.net.ssl.SSLSocketFactory;
import javax.net.ssl.X509TrustManager;

class ClientFactory {
    OkHttpClient defaultClient() {
        return new OkHttpClient.Builder().build();
    }

    OkHttpClient verifier() {
        return new OkHttpClient.Builder()
            .hostnameVerifier((hostname, session) -> hostname.equals(session.getPeerHost()))
            .build();
    }

    OkHttpClient trustManager(SSLSocketFactory socketFactory, X509TrustManager validatingManager) {
        return new OkHttpClient.Builder()
            .sslSocketFactory(socketFactory, validatingManager)
            .build();
    }
}
