package com.example;

import android.webkit.WebSettings;
import android.webkit.WebView;

class Page {
    void bind(WebView webView) {
        webView.getSettings().setAllowUniversalAccessFromFileURLs(true);
    }
}
