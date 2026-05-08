package com.example;

import android.webkit.WebSettings;
import android.webkit.WebView;

class BrowserActivity {
    void setupWebView(WebView webView) {
        webView.getSettings().setAllowFileAccessFromFileURLs(true);
    }
}
