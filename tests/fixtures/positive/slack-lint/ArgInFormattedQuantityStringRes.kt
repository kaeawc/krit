package fixtures.positive.slacklint

import android.content.Context

fun itemCount(context: Context, count: Int): String =
    context.resources.getQuantityString(1, count, count)
