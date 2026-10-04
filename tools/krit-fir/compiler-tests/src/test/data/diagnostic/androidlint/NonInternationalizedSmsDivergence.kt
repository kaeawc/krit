// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 13, 14
// Divergence (precision): a destination whose first character is a `+`
// written as a unicode escape. Go does not decode escape sequences before
// testing for the `+` prefix, so it reports both calls; the runtime value of
// each literal is "+1555", an E.164 destination, so the message is false and
// FIR does not report them.
package test

import android.telephony.SmsManager

fun escapedPlus(sms: SmsManager) {
    sms.sendTextMessage("\u002B1555", null, "Hi", null, null)
    sms.sendTextMessage("\u002b1555", null, "Hi", null, null)
}
