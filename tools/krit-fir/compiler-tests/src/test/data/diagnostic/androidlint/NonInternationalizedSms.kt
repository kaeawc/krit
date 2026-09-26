// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 10, 14, 15, 19, 20, 21, 25, 30, 36, 38, 50, 51, 52, 53, 54, 55, 56, 57, 58, 59
package test

import android.telephony.SmsManager

class SmsSender(private val manager: SmsManager) {
    fun local() {
        val sms = SmsManager.getDefault()
        <!NonInternationalizedSms!>sms.sendTextMessage("5551234567", null, "Your code: 1234", null, null)<!>
    }

    fun staticChain() {
        <!NonInternationalizedSms!>SmsManager.getDefault().sendTextMessage("5551234567", null, "Hi", null, null)<!>
        <!NonInternationalizedSms!>SmsManager.getSmsManagerForSubscriptionId(1).sendTextMessage("5551234567", null, "Hi", null, null)<!>
    }

    fun parameter(smsManager: SmsManager, parts: ArrayList<String>) {
        <!NonInternationalizedSms!>smsManager.sendMultipartTextMessage("5551234567", null, parts, null, null)<!>
        <!NonInternationalizedSms!>smsManager.sendTextMessage("5551234567", null, "Hi", null, null, 7L)<!>
        <!NonInternationalizedSms!>smsManager.sendMultipartTextMessage("5551234567", null, parts, null, null, 7L)<!>
    }

    fun classProperty() {
        <!NonInternationalizedSms!>manager.sendTextMessage("5551234567", null, "Hi", null, null)<!>
    }

    fun typedLocal() {
        val sms: SmsManager? = SmsManager.getDefault()
        <!NonInternationalizedSms!>sms?.sendTextMessage("5551234567", null, "Hi", null, null)<!>
    }

    // The finding sits on the first line of the whole call expression.
    fun multiLine() {
        val sms: SmsManager? = SmsManager.getDefault()
        <!NonInternationalizedSms!>sms<!>
            ?.sendTextMessage("5551234567", null, "Hi", null, null)
        <!NonInternationalizedSms!>SmsManager<!>.getDefault()
            .sendTextMessage(
                "5551234567",
                null,
                "Hi",
                null,
                null,
            )
    }

    // Literal values, read the way Go reads them.
    fun literals(sms: SmsManager) {
        <!NonInternationalizedSms!>sms.sendTextMessage("", null, "Hi", null, null)<!>
        <!NonInternationalizedSms!>sms.sendTextMessage(("5551234567"), null, "Hi", null, null)<!>
        <!NonInternationalizedSms!>sms.sendTextMessage(" +15551234567", null, "Hi", null, null)<!>
        <!NonInternationalizedSms!>sms.sendTextMessage("\t+15551234567", null, "Hi", null, null)<!>
        <!NonInternationalizedSms!>sms.sendTextMessage("\u0031555", null, "Hi", null, null)<!>
        <!NonInternationalizedSms!>sms.sendTextMessage("\\+1555", null, "Hi", null, null)<!>
        <!NonInternationalizedSms!>sms.sendTextMessage("\$1555", null, "Hi", null, null)<!>
        <!NonInternationalizedSms!>sms.sendTextMessage("""5551234567""", null, "Hi", null, null)<!>
        <!NonInternationalizedSms!>sms.sendTextMessage("""\u002B1555""", null, "Hi", null, null)<!>
        <!NonInternationalizedSms!>sms<!>.sendTextMessage("""
            +15551234567""", null, "Hi", null, null)
    }

    // E.164 destinations are fine, including through escapes and raw strings.
    fun international(sms: SmsManager) {
        sms.sendTextMessage("+15551234567", null, "Hi", null, null)
        sms.sendTextMessage(("+15551234567"), null, "Hi", null, null)
        sms.sendTextMessage("+\u0031555", null, "Hi", null, null)
        sms.sendTextMessage("+\n1555", null, "Hi", null, null)
        sms.sendTextMessage("""+\u0031555""", null, "Hi", null, null)
        sms.sendTextMessage("""+15551234567""", null, "Hi", null, null)
        SmsManager.getDefault().sendMultipartTextMessage("+15551234567", null, arrayListOf("Hi"), null, null)
    }

    // Dynamic destinations are never reported.
    fun dynamic(sms: SmsManager, dest: String, area: String) {
        sms.sendTextMessage(dest, null, "Hi", null, null)
        sms.sendTextMessage("555$area", null, "Hi", null, null)
        sms.sendTextMessage("${"555"}", null, "Hi", null, null)
        sms.sendTextMessage("555" + area, null, "Hi", null, null)
        sms.sendTextMessage(LOCAL_NUMBER, null, "Hi", null, null)
        sms.sendTextMessage("""
            5551234567""".trimIndent(), null, "Hi", null, null)
    }

    // Only the destination is read; the message body may be anything.
    fun otherArguments(sms: SmsManager, dest: String) {
        sms.sendTextMessage(dest, "5551234567", "5551234567", null, null)
        sms.sendDataMessage("5551234567", null, 80, byteArrayOf(), null, null)
    }
}

const val LOCAL_NUMBER = "5551234567"
