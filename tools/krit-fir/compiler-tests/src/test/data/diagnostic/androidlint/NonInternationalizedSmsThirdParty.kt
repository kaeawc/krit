// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 38, 39
// A third-party or project class whose simple name is SmsManager counts in
// any package, as Go matches it by that name (like SetJavaScriptEnabled keeps
// third-party WebSettings classes). A vendor SmsManager that forwards to the
// platform still sends an SMS to a destination that is not E.164.
// - Members of a class, of its companion object, and of an object named
//   SmsManager.
// Other classes' same-named methods are left alone by both.
// Divergence (recall): a nested object named SmsManager reached through its
// outer name, `Telephony.SmsManager.sendTextMessage(...)`. Go needs the
// receiver path to start with `SmsManager` and misses it; FIR resolves the
// SmsManager member.
package com.vendor.telephony

class SmsManager private constructor() {
    fun sendTextMessage(destinationAddress: String, scAddress: String?, text: String, a: Any?, b: Any?) {
        android.telephony.SmsManager.getDefault().sendTextMessage(destinationAddress, scAddress, text, null, null)
    }

    companion object {
        fun getDefault() = SmsManager()
        fun sendMultipartTextMessage(destinationAddress: String, parts: List<String>) {}
    }
}

object Telephony {
    object SmsManager {
        fun sendTextMessage(destinationAddress: String, text: String) {}
    }
}

class Other {
    fun sendTextMessage(destinationAddress: String, text: String) {}
}

fun send(other: Other) {
    <!NonInternationalizedSms!>SmsManager.getDefault().sendTextMessage("5551234567", null, "Hi", null, null)<!>
    <!NonInternationalizedSms!>SmsManager.sendMultipartTextMessage("5551234567", listOf("Hi"))<!>
    <!NonInternationalizedSms!>Telephony.SmsManager.sendTextMessage("5551234567", "Hi")<!>
    SmsManager.getDefault().sendTextMessage("+15551234567", null, "Hi", null, null)
    other.sendTextMessage("5551234567", "Hi")
}
