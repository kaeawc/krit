// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
package test

class NotificationManager
class Context {
    fun getSystemService(name: String): Any = NotificationManager()
}

const val NOTIFICATION_SERVICE = "notification"

// UnsafeCast.kt line 126: this is a local lookalike, not android.content.Context.
// Go treats the same-file Context as a project lookalike and FIR resolves its non-null Any return, so neither reports.
fun service(context: Context) {
    val notificationManager = context.getSystemService(NOTIFICATION_SERVICE) as NotificationManager
    println(notificationManager)
}
