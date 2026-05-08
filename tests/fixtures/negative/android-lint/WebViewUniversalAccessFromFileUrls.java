package com.example;

import android.webkit.WebSettings;
import android.webkit.WebView;

class Page {
    void bind(WebView webView) {
        webView.getSettings().setAllowUniversalAccessFromFileURLs(false);
    }
}

class FakeSettings {
    void setAllowUniversalAccessFromFileURLs(boolean value) {}
}

class Caller {
    void doIt() {
        FakeSettings s = new FakeSettings();
        s.setAllowUniversalAccessFromFileURLs(true);
    }
}
