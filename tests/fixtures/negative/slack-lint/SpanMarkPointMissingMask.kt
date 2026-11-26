package fixtures.negative.slacklint

import android.text.Spanned

fun isExclusive(flags: Int): Boolean = flags and Spanned.SPAN_POINT_MARK_MASK == Spanned.SPAN_EXCLUSIVE_EXCLUSIVE

fun isComposing(flags: Int): Boolean = flags == Spanned.SPAN_COMPOSING
