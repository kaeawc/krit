// Compiler-test source stub; never packaged in the production artifact.
package android.webkit;

import android.net.http.SslError;

public class WebViewClient {
    public WebViewClient() {
    }

    public boolean shouldOverrideUrlLoading(WebView view, WebResourceRequest request) {
        throw new RuntimeException("Stub!");
    }

    public void onPageFinished(WebView view, String url) {
        throw new RuntimeException("Stub!");
    }

    public void onReceivedSslError(WebView view, SslErrorHandler handler, SslError error) {
        throw new RuntimeException("Stub!");
    }
}
