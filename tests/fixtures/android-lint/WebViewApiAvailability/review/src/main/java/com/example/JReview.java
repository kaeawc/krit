package com.example;

import android.os.Build;
import android.webkit.WebView;
import android.webkit.WebViewClient;

public class JReview {
    // The else branch runs below API 26.
    WebViewClient throwThenElse(WebView webView) {
        if (Build.VERSION.SDK_INT >= 26) throw new IllegalStateException("use compat");
        else {
            return webView.getWebViewClient();
        }
    }

    // The else branch runs at API 26 and up.
    void throwBelowElse(WebView webView) {
        if (Build.VERSION.SDK_INT < 26) throw new IllegalStateException("too old");
        else {
            webView.getWebChromeClient();
        }
    }

    WebViewClient ternary(WebView webView) {
        return Build.VERSION.SDK_INT >= 26 ? webView.getWebViewClient() : null;
    }

    void reversed(WebView webView) {
        if (26 <= Build.VERSION.SDK_INT) {
            webView.getWebChromeClient();
        }
    }
}
