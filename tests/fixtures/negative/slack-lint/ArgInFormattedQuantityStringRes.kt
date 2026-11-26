package fixtures.negative.slacklint

import android.content.Context

fun getFormattedCount(count: Int): String = count.toString()

fun itemCount(context: Context, count: Int): String =
    context.resources.getQuantityString(1, count, getFormattedCount(count))

interface Strings { fun getQuantityString(id: Int, count: Int, vararg args: Any): String }

fun wrapped(strings: Strings, count: Int) = strings.getQuantityString(1, count, count)
