package com.example

import android.telephony.SmsManager

class SmsSender {
    fun sendVerificationCode() {
        val sms = SmsManager.getDefault()
        sms.sendTextMessage("5551234567", null, "Your code: 1234", null, null)
    }
}
