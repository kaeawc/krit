package com.example;

import android.webkit.JavascriptInterface;

public class HiddenBridge {
    @JavascriptInterface
    String token() {
        return "";
    }
}
