package com.example;

import android.os.Build;
import android.webkit.WebView;
import java.util.List;

class JReview2 {
    private Fake webView;

    // The loop variable binds in the body, not in the iterable: there webView is the Fake field.
    void iterable() {
        if (Build.VERSION.SDK_INT >= 26) {
            for (WebView webView : views(webView.getWebChromeClient())) {
                webView.getWebViewClient();
            }
        }
    }

    List<WebView> views(Object client) {
        return null;
    }
}
