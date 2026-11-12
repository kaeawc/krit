// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 19, 20, 26
// Flags that Go reads differently from the compiler.
package test.implicitpendingintentdivergence

import android.app.PendingIntent
import android.content.Context
import android.content.Intent

object Flags {
    val FLAG_EXTRA = 1
    const val BASE = PendingIntent.FLAG_UPDATE_CURRENT
    const val PLAIN = BASE or PendingIntent.FLAG_ONE_SHOT
    const val SAFE = BASE or PendingIntent.FLAG_IMMUTABLE
}

fun literalMutability(context: Context, intent: Intent) {
    // Go reports this because the literal is not a FLAG_IMMUTABLE reference; 67108864 is FLAG_IMMUTABLE.
    PendingIntent.getActivity(context, 0, intent, 67108864)
    PendingIntent.getActivity(context, 0, intent, PendingIntent.FLAG_UPDATE_CURRENT or 33554432)
}

fun foreignConstant(context: Context, intent: Intent) {
    // Go reports this because the identifier contains FLAG_; the non-const Flags.FLAG_EXTRA has no value the
    // checker can read, so the call is left alone.
    PendingIntent.getActivity(context, 0, intent, Flags.FLAG_EXTRA)
}

fun constants(context: Context, intent: Intent) {
    // Go misses this: the identifier Flags.PLAIN has no FLAG_ in its name, so it sees no flag constant. The
    // const folds to FLAG_UPDATE_CURRENT or FLAG_ONE_SHOT, which has no mutability flag.
    <!ImplicitPendingIntent!>PendingIntent.getActivity(context, 0, intent, Flags.PLAIN)<!>
    PendingIntent.getActivity(context, 0, intent, Flags.SAFE)
}
