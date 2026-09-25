// Compiler-test source stub; never packaged in the production artifact.
package android.os;

public interface Parcelable {
    int CONTENTS_FILE_DESCRIPTOR = 1;

    int PARCELABLE_WRITE_RETURN_VALUE = 1;

    // Abstract in the SDK. They are default methods here only because the
    // kotlin-parcelize compiler plugin (which generates them for @Parcelize
    // classes) is not loaded in compiler-tests; hand-written Parcelables still
    // override them exactly as real code does.
    default int describeContents() {
        throw new RuntimeException("Stub!");
    }

    default void writeToParcel(Parcel dest, int flags) {
        throw new RuntimeException("Stub!");
    }

    public static interface Creator<T> {
        T createFromParcel(Parcel source);

        T[] newArray(int size);
    }
}
