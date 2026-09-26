// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 34
// Divergence (recall): platform SmsManager sends to a non-E.164 literal that
// Go misses, each a true positive FIR resolves. Go needs the receiver's
// identifiers to start with `SmsManager`, the receiver's simple name to be
// `smsManager`, or a same-named declaration in an enclosing function or class
// (or the file) that mentions `SmsManager`, and it needs an explicit receiver.
// - A fully qualified android.telephony.SmsManager and an import alias.
// - Receivers that are calls or `!!` expressions: getSystemService,
//   createForSubscriptionId, and a not-null assertion; a lambda's `it`,
//   which has no declaration Go can find.
// - A property inherited from a base class declared in another class.
// - Implicit receivers of scope functions (`with`, `apply`).
// The one Go finding here is the unaliased `SmsManager.getDefault()` chain.
package test

import android.content.Context
import android.telephony.SmsManager
import android.telephony.SmsManager as Sms

open class Base {
    val sms: SmsManager = SmsManager.getDefault()
}

class Derived : Base() {
    fun inherited() {
        <!NonInternationalizedSms!>sms.sendTextMessage("5551234567", null, "Hi", null, null)<!>
    }
}

fun qualified() {
    <!NonInternationalizedSms!>android.telephony.SmsManager.getDefault().sendTextMessage("5551234567", null, "Hi", null, null)<!>
    <!NonInternationalizedSms!>Sms.getDefault().sendTextMessage("5551234567", null, "Hi", null, null)<!>
    <!NonInternationalizedSms!>SmsManager.getDefault().sendTextMessage("5551234567", null, "Hi", null, null)<!>
}

fun callReceivers(context: Context, base: SmsManager, maybe: SmsManager?) {
    <!NonInternationalizedSms!>context.getSystemService(SmsManager::class.java).sendTextMessage("5551234567", null, "Hi", null, null)<!>
    <!NonInternationalizedSms!>base.createForSubscriptionId(2).sendTextMessage("5551234567", null, "Hi", null, null)<!>
    <!NonInternationalizedSms!>maybe!!.sendTextMessage("5551234567", null, "Hi", null, null)<!>
    maybe?.let { <!NonInternationalizedSms!>it.sendTextMessage("5551234567", null, "Hi", null, null)<!> }
}

fun implicitReceivers(manager: SmsManager) {
    with(manager) {
        <!NonInternationalizedSms!>sendTextMessage("5551234567", null, "Hi", null, null)<!>
    }
    manager.apply {
        <!NonInternationalizedSms!>sendMultipartTextMessage("5551234567", null, arrayListOf("Hi"), null, null)<!>
    }
}
