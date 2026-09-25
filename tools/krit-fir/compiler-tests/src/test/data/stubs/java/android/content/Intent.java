// Compiler-test source stub; never packaged in the production artifact.
package android.content;

import android.net.Uri;
import android.os.Bundle;
import android.os.Parcelable;

public class Intent implements Parcelable, Cloneable {
    public static final String ACTION_BOOT_COMPLETED = "android.intent.action.BOOT_COMPLETED";

    public static final String ACTION_MAIN = "android.intent.action.MAIN";

    public static final String ACTION_SEND = "android.intent.action.SEND";

    public static final String ACTION_VIEW = "android.intent.action.VIEW";

    public static final String CATEGORY_LAUNCHER = "android.intent.category.LAUNCHER";

    public static final String EXTRA_TEXT = "android.intent.extra.TEXT";

    public static final int FLAG_ACTIVITY_CLEAR_TOP = 67108864;

    public static final int FLAG_ACTIVITY_NEW_TASK = 268435456;

    public Intent() {
    }

    public Intent(Intent o) {
    }

    public Intent(String action) {
    }

    public Intent(String action, Uri uri) {
    }

    public Intent(Context packageContext, Class<?> cls) {
    }

    public static Intent createChooser(Intent target, CharSequence title) {
        throw new RuntimeException("Stub!");
    }

    public String getAction() {
        throw new RuntimeException("Stub!");
    }

    public Intent setAction(String action) {
        throw new RuntimeException("Stub!");
    }

    public Uri getData() {
        throw new RuntimeException("Stub!");
    }

    public Intent setData(Uri data) {
        throw new RuntimeException("Stub!");
    }

    public Bundle getExtras() {
        throw new RuntimeException("Stub!");
    }

    public String getStringExtra(String name) {
        throw new RuntimeException("Stub!");
    }

    public int getIntExtra(String name, int defaultValue) {
        throw new RuntimeException("Stub!");
    }

    public boolean getBooleanExtra(String name, boolean defaultValue) {
        throw new RuntimeException("Stub!");
    }

    public boolean hasExtra(String name) {
        throw new RuntimeException("Stub!");
    }

    public Intent putExtra(String name, String value) {
        throw new RuntimeException("Stub!");
    }

    public Intent putExtra(String name, int value) {
        throw new RuntimeException("Stub!");
    }

    public Intent putExtra(String name, boolean value) {
        throw new RuntimeException("Stub!");
    }

    public Intent putExtra(String name, Parcelable value) {
        throw new RuntimeException("Stub!");
    }

    public Intent addFlags(int flags) {
        throw new RuntimeException("Stub!");
    }

    public Intent setPackage(String packageName) {
        throw new RuntimeException("Stub!");
    }

    public Intent setClass(Context packageContext, Class<?> cls) {
        throw new RuntimeException("Stub!");
    }

    public int describeContents() {
        throw new RuntimeException("Stub!");
    }

    public void writeToParcel(android.os.Parcel out, int flags) {
        throw new RuntimeException("Stub!");
    }
}
