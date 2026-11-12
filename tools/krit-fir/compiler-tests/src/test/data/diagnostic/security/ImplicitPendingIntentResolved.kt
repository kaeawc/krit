// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
// Go matches the spelling `PendingIntent.<factory>(`; the callee resolves to
// android.app.PendingIntent through an import alias or a static import.
package test.implicitpendingintentresolved

import android.app.PendingIntent as Pending
import android.app.PendingIntent.getBroadcast
import android.app.PendingIntent.FLAG_UPDATE_CURRENT
import android.content.Context
import android.content.Intent

fun aliased(context: Context, intent: Intent) {
    <!ImplicitPendingIntent!>Pending.getActivity(context, 0, intent, Pending.FLAG_UPDATE_CURRENT)<!>
}

fun staticImport(context: Context, intent: Intent) {
    <!ImplicitPendingIntent!>getBroadcast(context, 0, intent, FLAG_UPDATE_CURRENT)<!>
}
