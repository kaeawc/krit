// Smoke: resource lookups, styled attributes, and configuration checks.
package stubs

import android.content.Context
import android.content.res.Configuration
import android.content.res.Resources
import android.content.res.TypedArray
import android.util.AttributeSet

fun readStyled(context: Context, attrs: AttributeSet?) {
    val res: Resources = context.resources
    val title: String = res.getString(android.R.string.ok)
    val formatted: String = res.getString(android.R.string.ok, "arg")
    val plural: String = res.getQuantityString(1, 2, 2)
    val id: Int = res.getIdentifier("ok", "string", "android")
    val night = res.configuration.uiMode and Configuration.UI_MODE_NIGHT_MASK == Configuration.UI_MODE_NIGHT_YES
    val landscape = res.configuration.orientation == Configuration.ORIENTATION_LANDSCAPE
    val typed: TypedArray = context.obtainStyledAttributes(attrs, intArrayOf(1))
    typed.use { array ->
        println(array.getString(0))
        println(array.getInt(0, 0) + array.getColor(0, 0))
        println(array.getBoolean(0, false))
    }
    val color = try {
        res.getColor(1, context.theme)
    } catch (e: Resources.NotFoundException) {
        0
    }
    println("$title $formatted $plural $id $night $landscape $color")
}
