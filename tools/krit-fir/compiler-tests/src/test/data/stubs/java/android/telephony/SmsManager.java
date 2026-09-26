// Compiler-test source stub; never packaged in the production artifact.
package android.telephony;

import android.app.PendingIntent;
import java.util.ArrayList;
import java.util.List;

public final class SmsManager {
    SmsManager() {
    }

    @Deprecated
    public static SmsManager getDefault() {
        throw new RuntimeException("Stub!");
    }

    @Deprecated
    public static SmsManager getSmsManagerForSubscriptionId(int subId) {
        throw new RuntimeException("Stub!");
    }

    public SmsManager createForSubscriptionId(int subId) {
        throw new RuntimeException("Stub!");
    }

    public int getSubscriptionId() {
        throw new RuntimeException("Stub!");
    }

    public void sendTextMessage(
            String destinationAddress,
            String scAddress,
            String text,
            PendingIntent sentIntent,
            PendingIntent deliveryIntent) {
        throw new RuntimeException("Stub!");
    }

    public void sendTextMessage(
            String destinationAddress,
            String scAddress,
            String text,
            PendingIntent sentIntent,
            PendingIntent deliveryIntent,
            long messageId) {
        throw new RuntimeException("Stub!");
    }

    public ArrayList<String> divideMessage(String text) {
        throw new RuntimeException("Stub!");
    }

    public void sendMultipartTextMessage(
            String destinationAddress,
            String scAddress,
            ArrayList<String> parts,
            ArrayList<PendingIntent> sentIntents,
            ArrayList<PendingIntent> deliveryIntents) {
        throw new RuntimeException("Stub!");
    }

    public void sendMultipartTextMessage(
            String destinationAddress,
            String scAddress,
            List<String> parts,
            List<PendingIntent> sentIntents,
            List<PendingIntent> deliveryIntents,
            long messageId) {
        throw new RuntimeException("Stub!");
    }

    public void sendDataMessage(
            String destinationAddress,
            String scAddress,
            short destinationPort,
            byte[] data,
            PendingIntent sentIntent,
            PendingIntent deliveryIntent) {
        throw new RuntimeException("Stub!");
    }
}
