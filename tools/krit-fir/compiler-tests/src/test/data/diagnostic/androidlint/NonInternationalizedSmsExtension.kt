// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 33, 36, 37, 42, 46
// Project extensions named like the platform send methods, declared on
// SmsManager. Go reads the receiver and the first unlabeled argument, so it
// reports them like the platform members, and so does FIR (the destination is
// the argument bound to the first parameter).
// Divergence (named arguments, possible only on a Kotlin extension):
// - `text = ..., destination = "5551234567"` names the literal destination;
//   Go finds no unlabeled argument and misses it, FIR reports it.
// - `destination = dest, "5551234567"` passes the literal as the text; Go
//   reads it as the first unlabeled argument and reports it, although the
//   destination is dynamic, so FIR does not.
// Divergence (precision): an extension on String called on a receiver named
// `smsManager`; Go accepts the receiver by its name, but no SmsManager is
// involved.
package test

import android.telephony.SmsManager

typealias Sms = SmsManager

fun SmsManager.sendTextMessage(destination: String, text: String) {
    sendTextMessage(destination, null, text, null, null)
}

fun SmsManager?.sendMultipartTextMessage(destination: String, parts: List<String>) {}

fun Sms.sendTextMessage(destination: String) {}

fun String.sendTextMessage(destination: String, text: String) {}

fun extensions(sms: SmsManager, dest: String) {
    <!NonInternationalizedSms!>sms.sendTextMessage("5551234567", "Hi")<!>
    sms.sendTextMessage("+15551234567", "Hi")
    sms.sendTextMessage(dest, "5551234567")
    <!NonInternationalizedSms!>sms.sendMultipartTextMessage("5551234567", listOf("Hi"))<!>
    <!NonInternationalizedSms!>sms.sendTextMessage("5551234567")<!>
}

fun namedArguments(sms: SmsManager, dest: String) {
    <!NonInternationalizedSms!>sms.sendTextMessage(text = "Hi", destination = "5551234567")<!>
    sms.sendTextMessage(destination = dest, "5551234567")
}

fun otherReceiver(smsManager: String) {
    smsManager.sendTextMessage("5551234567", "Hi")
}
