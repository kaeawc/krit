// Compiler-test source stub; never packaged in the production artifact.
package android.webkit;

public abstract class WebSettings {
    public static final int MIXED_CONTENT_ALWAYS_ALLOW = 0;

    public static final int MIXED_CONTENT_NEVER_ALLOW = 1;

    public static final int MIXED_CONTENT_COMPATIBILITY_MODE = 2;

    public WebSettings() {
    }

    public abstract boolean getJavaScriptEnabled();

    public abstract void setJavaScriptEnabled(boolean flag);

    public abstract boolean getDomStorageEnabled();

    public abstract void setDomStorageEnabled(boolean flag);

    public abstract boolean getAllowFileAccess();

    public abstract void setAllowFileAccess(boolean allow);

    public abstract boolean getAllowContentAccess();

    public abstract void setAllowContentAccess(boolean allow);

    @Deprecated
    public abstract boolean getAllowFileAccessFromFileURLs();

    @Deprecated
    public abstract void setAllowFileAccessFromFileURLs(boolean flag);

    @Deprecated
    public abstract boolean getAllowUniversalAccessFromFileURLs();

    @Deprecated
    public abstract void setAllowUniversalAccessFromFileURLs(boolean flag);

    public abstract int getMixedContentMode();

    public abstract void setMixedContentMode(int mode);

    public abstract String getUserAgentString();

    public abstract void setUserAgentString(String ua);
}
