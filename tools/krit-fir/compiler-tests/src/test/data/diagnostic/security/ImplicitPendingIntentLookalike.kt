// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
// A project class named PendingIntent is not the Android class, and its factories
// have no mutability requirement.
package test.implicitpendingintentlookalike

object PendingIntent {
    const val FLAG_UPDATE_CURRENT = 134217728

    fun getActivity(requestCode: Int, intent: Any?, unused: Int, flags: Int): Int = requestCode + flags

    fun getBroadcast(requestCode: Int, intent: Any?, unused: Int, flags: Int): Int = requestCode + flags
}

fun lookalike() {
    PendingIntent.getActivity(0, null, 0, PendingIntent.FLAG_UPDATE_CURRENT)
    PendingIntent.getBroadcast(0, null, 0, 0)
}
