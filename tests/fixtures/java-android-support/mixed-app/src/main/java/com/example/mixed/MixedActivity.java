package com.example.mixed;

import android.app.Activity;
import android.os.Bundle;
import android.webkit.WebView;

public final class MixedActivity extends Activity {
    @Override
    protected void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);
        KotlinApi api = new KotlinApi();
        api.value();

        WebView webView = new WebView(this);
        webView.addJavascriptInterface(new Object(), "bridge");
    }
}
