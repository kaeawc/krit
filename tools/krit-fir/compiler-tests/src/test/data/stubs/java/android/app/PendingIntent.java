// Compiler-test source stub; never packaged in the production artifact.
package android.app;

import android.content.Context;
import android.content.Intent;
import android.os.Bundle;
import android.os.Parcelable;

public final class PendingIntent implements Parcelable {
    public static final int FLAG_ONE_SHOT = 1073741824;

    public static final int FLAG_NO_CREATE = 536870912;

    public static final int FLAG_CANCEL_CURRENT = 268435456;

    public static final int FLAG_UPDATE_CURRENT = 134217728;

    public static final int FLAG_IMMUTABLE = 67108864;

    public static final int FLAG_MUTABLE = 33554432;

    PendingIntent() {
    }

    public static PendingIntent getActivity(Context context, int requestCode, Intent intent, int flags) {
        throw new RuntimeException("Stub!");
    }

    public static PendingIntent getActivity(Context context, int requestCode, Intent intent, int flags, Bundle options) {
        throw new RuntimeException("Stub!");
    }

    public static PendingIntent getActivities(Context context, int requestCode, Intent[] intents, int flags) {
        throw new RuntimeException("Stub!");
    }

    public static PendingIntent getActivities(Context context, int requestCode, Intent[] intents, int flags, Bundle options) {
        throw new RuntimeException("Stub!");
    }

    public static PendingIntent getBroadcast(Context context, int requestCode, Intent intent, int flags) {
        throw new RuntimeException("Stub!");
    }

    public static PendingIntent getService(Context context, int requestCode, Intent intent, int flags) {
        throw new RuntimeException("Stub!");
    }

    public static PendingIntent getForegroundService(Context context, int requestCode, Intent intent, int flags) {
        throw new RuntimeException("Stub!");
    }

    public void cancel() {
        throw new RuntimeException("Stub!");
    }
}
