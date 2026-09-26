// Compiler-test source stub; never packaged in the production artifact.
package android.content.pm;

import android.os.Parcelable;

public final class SigningInfo implements Parcelable {
    public SigningInfo() {
    }

    public boolean hasMultipleSigners() {
        throw new RuntimeException("Stub!");
    }

    public boolean hasPastSigningCertificates() {
        throw new RuntimeException("Stub!");
    }

    public Signature[] getSigningCertificateHistory() {
        throw new RuntimeException("Stub!");
    }

    public Signature[] getApkContentsSigners() {
        throw new RuntimeException("Stub!");
    }
}
