// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 19, 23
// Divergence (precision): a local that shadows an SmsManager property. Go
// looks for a declaration of the receiver's name in each enclosing function
// and class, and the class property's declaration mentions SmsManager, so it
// reports the call on the shadowing local, which is a project messenger.
// The unshadowed call reaches the platform SmsManager and both report it.
package test

import android.telephony.SmsManager

class Messenger {
    fun sendTextMessage(destination: String, sc: String?, text: String, a: Any?, b: Any?) {}
}

class Holder(private val sms: SmsManager) {
    fun shadowed() {
        val sms = Messenger()
        sms.sendTextMessage("5551234567", null, "Hi", null, null)
    }

    fun notShadowed() {
        <!NonInternationalizedSms!>sms.sendTextMessage("5551234567", null, "Hi", null, null)<!>
    }
}
