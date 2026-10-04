// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 21, 22, 23, 24
// Properties of function type named like the platform send methods, invoked
// on an SmsManager like a function. Go reads the call's name and receiver, so
// it reports them like extension functions, and so does FIR: the property is
// declared on SmsManager, or its explicit receiver is one.
// Neither reports the same property on a receiver of another type, or a local
// function value named like a send method.
// Divergence (recall): a parenthesized property invoked as a value,
// `(sms.sendTextMessage)(...)`. Go reads no call name through the parentheses
// and misses it; it is the same send to a destination that is not E.164.
package test

import android.telephony.SmsManager

val SmsManager.sendTextMessage: (String) -> Unit get() = {}

val Any.sendMultipartTextMessage: (String, String) -> Unit get() = { _, _ -> }

fun functionTypeProperty(sms: SmsManager, maybe: SmsManager?, text: String) {
    <!NonInternationalizedSms!>sms.sendTextMessage("5551234567")<!>
    <!NonInternationalizedSms!>sms.sendMultipartTextMessage("5551234567", "Hi")<!>
    <!NonInternationalizedSms!>maybe?.sendTextMessage("5551234567")<!>
    <!NonInternationalizedSms!>sms<!>
        .sendTextMessage("5551234567")
    sms.sendTextMessage("+15551234567")
    text.sendMultipartTextMessage("5551234567", "Hi")
    <!NonInternationalizedSms!>(sms.sendTextMessage)("5551234567")<!>
}

fun localValue() {
    val sendTextMessage: (String) -> Unit = {}
    sendTextMessage("5551234567")
}
