package fixtures.positive.slacklint

import android.content.Context
import android.view.View

class BadgeView(context: Context) : View(context) {
    override fun toString(): String = "BadgeView"
}

fun describe(badge: BadgeView): String = badge.toString()
