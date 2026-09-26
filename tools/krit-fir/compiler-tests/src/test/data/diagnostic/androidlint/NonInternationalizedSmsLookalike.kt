// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 30, 32, 36, 37, 42
// Divergence (precision): calls Go reports that do not reach the platform
// android.telephony.SmsManager, so no SMS destination is involved. Go cannot
// resolve the receiver and accepts one spelled starting with `SmsManager`,
// one named `smsManager`, or a name whose declaration in an enclosing
// function or class mentions `SmsManager`.
// - A project class named SmsManager (this file imports no platform class).
// - A `smsManager` receiver of a project messenger type.
// - A variable whose initializer mentions SmsManager but holds a project
//   messenger.
// Receivers with other names and no SmsManager mention are left alone by
// both, like Go's `messageService.sendTextMessage` negative.
package test

class SmsManager {
    fun sendTextMessage(destination: String, sc: String?, text: String, a: Any?, b: Any?) {}

    companion object {
        fun getDefault(): SmsManager = SmsManager()
    }
}

class Messenger(val backend: Any?) {
    fun sendTextMessage(destination: String, sc: String?, text: String, a: Any?, b: Any?) {}
    fun sendMultipartTextMessage(destination: String, sc: String?, parts: List<String>, a: Any?, b: Any?) {}
}

fun projectClass() {
    SmsManager.getDefault().sendTextMessage("5551234567", null, "Hi", null, null)
    val sms = SmsManager()
    sms.sendTextMessage("5551234567", null, "Hi", null, null)
}

fun namedLikeTheManager(smsManager: Messenger) {
    smsManager.sendTextMessage("5551234567", null, "Hi", null, null)
    smsManager.sendMultipartTextMessage("5551234567", null, listOf("Hi"), null, null)
}

fun mentionsTheManager() {
    val gateway = Messenger(SmsManager.getDefault())
    gateway.sendTextMessage("5551234567", null, "Hi", null, null)
}

fun otherNames(messageService: Messenger) {
    messageService.sendTextMessage("5551234567", null, "Hi", null, null)
}

fun sendTextMessage(destination: String, sc: String?, text: String, a: Any?, b: Any?) {}

fun topLevelFunction() {
    sendTextMessage("5551234567", null, "Hi", null, null)
}
