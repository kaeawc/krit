package com.example;

import android.webkit.RenderProcessGoneDetail;
import android.webkit.WebView;
import android.webkit.WebViewClient;

class JavaNoHandler extends WebViewClient {
}

class JavaHandles extends WebViewClient {
    @Override
    public boolean onRenderProcessGone(WebView view, RenderProcessGoneDetail detail) {
        return true;
    }
}

class JavaUser {
    void set(WebView view) {
        view.setWebViewClient(new WebViewClient());
        view.setWebViewClient(new WebViewClient() {});
    }
}
