// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 19, 20, 21, 22
// Project extensions named like the platform send methods, declared on Any or
// on a type parameter and called on an explicit SmsManager receiver. Go reads
// the receiver, not the callee, so it reports them, and so does FIR: the
// explicit receiver is an SmsManager.
// Neither reports them on a receiver of another type. On an implicit receiver
// Go needs a navigation expression and FIR reads only an explicit receiver
// for a function not declared on SmsManager, so neither reports that either.
package test

import android.telephony.SmsManager

fun Any.sendTextMessage(destination: String) {}

fun <T> T.sendMultipartTextMessage(destination: String) {}

fun anyReceiver(sms: SmsManager, maybe: SmsManager?, text: String) {
    <!NonInternationalizedSms!>sms.sendTextMessage("5551234567")<!>
    <!NonInternationalizedSms!>sms.sendMultipartTextMessage("5551234567")<!>
    <!NonInternationalizedSms!>maybe?.sendTextMessage("5551234567")<!>
    <!NonInternationalizedSms!>maybe.sendMultipartTextMessage("5551234567")<!>
    sms.sendTextMessage("+15551234567")
    text.sendTextMessage("5551234567")
    text.sendMultipartTextMessage("5551234567")
    with(sms) {
        sendTextMessage("5551234567")
    }
}
