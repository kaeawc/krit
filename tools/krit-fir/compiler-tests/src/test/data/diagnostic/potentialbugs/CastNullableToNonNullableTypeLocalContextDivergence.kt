// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 16
package test

class NotificationManager
class Context {
    fun getSystemService(name: String): Any = NotificationManager()
}

const val NOTIFICATION_SERVICE = "notification"

// UnsafeCast.kt line 126: this is a local lookalike, not android.content.Context.
// Go applies its Context.getSystemService platform-type heuristic and reports a false positive.
// FIR resolves the declared non-null Any return type, so there must be no finding here.
fun service(context: Context) {
    val notificationManager = context.getSystemService(NOTIFICATION_SERVICE) as NotificationManager
    println(notificationManager)
}
