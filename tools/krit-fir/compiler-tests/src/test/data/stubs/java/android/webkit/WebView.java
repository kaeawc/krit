// Compiler-test source stub; never packaged in the production artifact.
package android.webkit;

import android.content.Context;
import android.util.AttributeSet;
import android.widget.AbsoluteLayout;
import java.util.Map;

public class WebView extends AbsoluteLayout {
    public WebView(Context context) {
        super(context);
    }

    public WebView(Context context, AttributeSet attrs) {
        super(context, attrs);
    }

    public WebSettings getSettings() {
        throw new RuntimeException("Stub!");
    }

    public WebViewClient getWebViewClient() {
        throw new RuntimeException("Stub!");
    }

    public void setWebViewClient(WebViewClient client) {
        throw new RuntimeException("Stub!");
    }

    public void loadUrl(String url) {
        throw new RuntimeException("Stub!");
    }

    public void loadUrl(String url, Map<String, String> additionalHttpHeaders) {
        throw new RuntimeException("Stub!");
    }

    public void addJavascriptInterface(Object object, String name) {
        throw new RuntimeException("Stub!");
    }

    public void evaluateJavascript(String script, ValueCallback<String> resultCallback) {
        throw new RuntimeException("Stub!");
    }

    public void destroy() {
        throw new RuntimeException("Stub!");
    }

    protected void onLayout(boolean changed, int l, int t, int r, int b) {
        throw new RuntimeException("Stub!");
    }
}
