// Compiler-test source stub; never packaged in the production artifact.
package android.net;

import android.os.Parcelable;
import java.io.File;
import java.util.List;

public abstract class Uri implements Parcelable, Comparable<Uri> {
    // A static final field initialized at runtime: not a compile-time constant.
    public static final Uri EMPTY = null;

    Uri() {
    }

    public static Uri parse(String uriString) {
        throw new RuntimeException("Stub!");
    }

    public static Uri fromFile(File file) {
        throw new RuntimeException("Stub!");
    }

    public static String encode(String s) {
        throw new RuntimeException("Stub!");
    }

    public abstract String getScheme();

    public abstract String getHost();

    public abstract String getPath();

    public abstract String getLastPathSegment();

    public abstract List<String> getPathSegments();

    public String getQueryParameter(String key) {
        throw new RuntimeException("Stub!");
    }

    public abstract Builder buildUpon();

    public int compareTo(Uri other) {
        throw new RuntimeException("Stub!");
    }

    public static final class Builder {
        public Builder() {
        }

        public Builder scheme(String scheme) {
            throw new RuntimeException("Stub!");
        }

        public Builder authority(String authority) {
            throw new RuntimeException("Stub!");
        }

        public Builder path(String path) {
            throw new RuntimeException("Stub!");
        }

        public Builder appendPath(String newSegment) {
            throw new RuntimeException("Stub!");
        }

        public Builder appendQueryParameter(String key, String value) {
            throw new RuntimeException("Stub!");
        }

        public Uri build() {
            throw new RuntimeException("Stub!");
        }
    }
}
