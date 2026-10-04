// Compiler-test source stub; never packaged in the production artifact.
package android.content.pm;

public class PackageInfo {
    public String packageName;

    public String versionName;

    @Deprecated
    public Signature[] signatures;

    public SigningInfo signingInfo;

    public PackageInfo() {
    }

    public long getLongVersionCode() {
        throw new RuntimeException("Stub!");
    }
}
