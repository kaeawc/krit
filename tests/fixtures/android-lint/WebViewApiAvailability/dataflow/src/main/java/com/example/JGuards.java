package com.example;

import android.os.Build;
import android.webkit.WebView;
import java.util.List;

class JGuards {
    WebView webView;

    void booleanLocal() {
        boolean atLeastO = Build.VERSION.SDK_INT >= 26;
        if (atLeastO) {
            webView.getWebChromeClient();
        }
    }

    void anonymousClass() {
        if (Build.VERSION.SDK_INT >= 26) {
            Runnable r = new Runnable() {
                @Override
                public void run() {
                    webView.getWebChromeClient();
                }
            };
            r.run();
        }
    }

    void lambda() {
        if (Build.VERSION.SDK_INT >= 26) {
            Runnable r = () -> webView.getWebChromeClient();
            r.run();
        }
    }

    // Another method's loop variable must not hide the field here.
    void otherLoop(List<Fake> fakes) {
        for (Fake webView : fakes) {
            webView.getWebChromeClient();
        }
    }

    void fieldAfterLoop() {
        if (Build.VERSION.SDK_INT >= 26) {
            webView.getWebChromeClient();
        }
    }
}
