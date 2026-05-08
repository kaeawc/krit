package com.example

import android.telephony.SmsManager

class SmsSender {
    fun sendVerificationCode() {
        val sms = SmsManager.getDefault()
        sms.sendTextMessage("+15551234567", null, "Your code: 1234", null, null)
    }

    fun sendTo(internationalNumber: String, body: String) {
        SmsManager.getDefault().sendTextMessage(internationalNumber, null, body, null, null)
    }
}
