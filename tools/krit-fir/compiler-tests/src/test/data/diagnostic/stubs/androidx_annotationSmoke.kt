// Smoke: androidx annotations on the declaration sites their @Target allows.
package stubs

import android.os.Build
import androidx.annotation.CallSuper
import androidx.annotation.CheckResult
import androidx.annotation.ChecksSdkIntAtLeast
import androidx.annotation.ColorInt
import androidx.annotation.ColorRes
import androidx.annotation.DrawableRes
import androidx.annotation.FloatRange
import androidx.annotation.IdRes
import androidx.annotation.IntDef
import androidx.annotation.IntRange
import androidx.annotation.LayoutRes
import androidx.annotation.MainThread
import androidx.annotation.RequiresApi
import androidx.annotation.RequiresPermission
import androidx.annotation.StringRes
import androidx.annotation.UiThread
import androidx.annotation.VisibleForTesting
import androidx.annotation.WorkerThread

const val MODE_ON = 1
const val MODE_OFF = 0

@IntDef(MODE_ON, MODE_OFF)
@Retention(AnnotationRetention.SOURCE)
annotation class Mode

@RequiresApi(Build.VERSION_CODES.O)
class OreoApi @RequiresApi(26) constructor() {
    @RequiresPermission(android.Manifest.permission.CAMERA)
    fun capture() {}

    @RequiresPermission(
        anyOf = [android.Manifest.permission.ACCESS_FINE_LOCATION, android.Manifest.permission.ACCESS_COARSE_LOCATION],
    )
    fun locate() {}
}

open class SuperCalled {
    @CallSuper
    open fun onStart() {}
}

@MainThread
class UiThing(
    @StringRes val title: Int,
    @DrawableRes val icon: Int,
    @LayoutRes val layout: Int,
    @ColorRes val colorRes: Int,
    @IdRes val id: Int,
    @ColorInt val color: Int,
    @IntRange(from = 0, to = 10) val count: Int,
    @FloatRange(from = 0.0, to = 1.0) val alpha: Float,
    @Mode val mode: Int,
) {
    @CheckResult
    fun copyWithCount(@IntRange(from = 0) newCount: Int): UiThing = this

    @CheckResult(suggest = "#copyWithCount")
    fun other(): UiThing = this

    @WorkerThread
    fun load() {}

    @UiThread
    fun render() {}

    @VisibleForTesting(otherwise = VisibleForTesting.PRIVATE)
    internal fun exposed() {}

    @get:ChecksSdkIntAtLeast(api = Build.VERSION_CODES.O)
    val isOreo: Boolean get() = Build.VERSION.SDK_INT >= Build.VERSION_CODES.O

    @ChecksSdkIntAtLeast(parameter = 0)
    fun isAtLeast(api: Int): Boolean = Build.VERSION.SDK_INT >= api

    @ColorInt
    fun tint(@ColorRes res: Int): Int = res
}
