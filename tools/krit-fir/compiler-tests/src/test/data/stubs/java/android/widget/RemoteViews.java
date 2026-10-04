// Compiler-test source stub; never packaged in the production artifact.
package android.widget;

import android.os.Parcel;
import android.os.Parcelable;

// The SDK class also implements LayoutInflater.Filter, which is not modeled.
public class RemoteViews implements Parcelable {
    public RemoteViews(String packageName, int layoutId) {
    }

    public void setTextViewText(int viewId, CharSequence text) {
        throw new RuntimeException("Stub!");
    }

    public int describeContents() {
        throw new RuntimeException("Stub!");
    }

    public void writeToParcel(Parcel dest, int flags) {
        throw new RuntimeException("Stub!");
    }
}
