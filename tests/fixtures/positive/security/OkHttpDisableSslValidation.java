package test;

import okhttp3.OkHttpClient;
import javax.net.ssl.SSLSocketFactory;
import javax.net.ssl.X509TrustManager;

class ClientFactory {
    OkHttpClient verifier() {
        return new OkHttpClient.Builder()
            .hostnameVerifier((hostname, session) -> true)
            .build();
    }

    OkHttpClient trustManager(SSLSocketFactory socketFactory, X509TrustManager unsafeTrustManager) {
        return new OkHttpClient.Builder()
            .sslSocketFactory(socketFactory, unsafeTrustManager)
            .build();
    }
}
