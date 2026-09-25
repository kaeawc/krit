// Compiler-test source stub; never packaged in the production artifact.
package org.apache.http;

public interface HttpResponse {
    StatusLine getStatusLine();

    HttpEntity getEntity();
}
