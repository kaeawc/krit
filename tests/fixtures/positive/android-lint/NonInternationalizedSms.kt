package com.example

import android.telephony.SmsManager

class SmsSender {
    fun send(smsManager: SmsManager) {
        smsManager.sendTextMessage("+1234567890", null, "Hello", null, null)
    }
}
