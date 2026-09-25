// Compiler-test source stub; never packaged in the production artifact.
package org.apache.http.impl.client;

import java.io.IOException;
import org.apache.http.HttpResponse;
import org.apache.http.client.methods.HttpUriRequest;

@Deprecated
public class DefaultHttpClient extends AbstractHttpClient {
    public DefaultHttpClient() {
    }

    public HttpResponse execute(HttpUriRequest request) throws IOException {
        throw new RuntimeException("Stub!");
    }

    public void close() {
        throw new RuntimeException("Stub!");
    }
}
