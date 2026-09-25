// Compiler-test source stub; never packaged in the production artifact.
package android.os;

import java.util.ArrayList;

public final class Bundle extends BaseBundle implements Cloneable, Parcelable {
    public static final Bundle EMPTY = null;

    public Bundle() {
    }

    public Bundle(int capacity) {
    }

    public Bundle(Bundle b) {
    }

    @Deprecated
    public <T extends Parcelable> T getParcelable(String key) {
        throw new RuntimeException("Stub!");
    }

    public <T> T getParcelable(String key, Class<T> clazz) {
        throw new RuntimeException("Stub!");
    }

    public void putParcelable(String key, Parcelable value) {
        throw new RuntimeException("Stub!");
    }

    public Bundle getBundle(String key) {
        throw new RuntimeException("Stub!");
    }

    public void putBundle(String key, Bundle value) {
        throw new RuntimeException("Stub!");
    }

    public ArrayList<String> getStringArrayList(String key) {
        throw new RuntimeException("Stub!");
    }

    public int describeContents() {
        throw new RuntimeException("Stub!");
    }

    public void writeToParcel(Parcel parcel, int flags) {
        throw new RuntimeException("Stub!");
    }
}
