// Compiler-test source stub; never packaged in the production artifact.
package android.net.http;

import android.content.Context;
import java.io.IOException;
import org.apache.http.HttpResponse;
import org.apache.http.client.HttpClient;
import org.apache.http.client.methods.HttpUriRequest;

// Removed from the SDK in API 23.
@Deprecated
public final class AndroidHttpClient implements HttpClient {
    AndroidHttpClient() {
    }

    public static AndroidHttpClient newInstance(String userAgent) {
        throw new RuntimeException("Stub!");
    }

    public static AndroidHttpClient newInstance(String userAgent, Context context) {
        throw new RuntimeException("Stub!");
    }

    public HttpResponse execute(HttpUriRequest request) throws IOException {
        throw new RuntimeException("Stub!");
    }

    public void close() {
        throw new RuntimeException("Stub!");
    }
}
