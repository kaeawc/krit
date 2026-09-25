// Compiler-test source stub; never packaged in the production artifact.
package org.apache.http.client;

import java.io.IOException;
import org.apache.http.HttpResponse;
import org.apache.http.client.methods.HttpUriRequest;

public interface HttpClient {
    HttpResponse execute(HttpUriRequest request) throws IOException;
}
