// Compiler-test source stub; never packaged in the production artifact.
package android.webkit;

import android.net.Uri;

public interface WebResourceRequest {
    Uri getUrl();

    String getMethod();

    boolean isForMainFrame();
}
