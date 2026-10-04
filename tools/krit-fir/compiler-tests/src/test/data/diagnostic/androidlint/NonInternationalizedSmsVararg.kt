// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 13, 14
// A project extension on SmsManager whose destination is a vararg. Go reads
// the first unlabeled argument, the first element, and so does FIR. A spread
// array is not a literal for either.
package test

import android.telephony.SmsManager

fun SmsManager.sendTextMessage(vararg destinations: String) {}

fun varargDestination(sms: SmsManager) {
    <!NonInternationalizedSms!>sms.sendTextMessage("5551234567", "5550000000")<!>
    <!NonInternationalizedSms!>sms.sendTextMessage("5551234567")<!>
    sms.sendTextMessage("+15551234567", "5550000000")
    sms.sendTextMessage(*arrayOf("5551234567"))
    sms.sendTextMessage()
}
