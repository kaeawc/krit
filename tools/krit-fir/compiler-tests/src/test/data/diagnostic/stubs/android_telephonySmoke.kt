// Smoke: SmsManager lookup, message splitting, and sends to a caller-supplied number.
package stubs

import android.app.PendingIntent
import android.content.Context
import android.content.Intent
import android.os.Build
import android.telephony.SmsManager

fun smsManager(context: Context, subscriptionId: Int): SmsManager =
    if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.S) {
        context.getSystemService(SmsManager::class.java).createForSubscriptionId(subscriptionId)
    } else {
        @Suppress("DEPRECATION")
        SmsManager.getSmsManagerForSubscriptionId(subscriptionId)
    }

fun sendSms(context: Context, manager: SmsManager, destination: String, body: String) {
    val sent = PendingIntent.getBroadcast(context, 0, Intent("stubs.SMS_SENT"), PendingIntent.FLAG_IMMUTABLE)
    val parts = manager.divideMessage(body)
    if (parts.size > 1) {
        manager.sendMultipartTextMessage(destination, null, parts, null, null)
    } else {
        manager.sendTextMessage(destination, null, body, sent, null)
    }
}
