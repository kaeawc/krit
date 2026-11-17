package p

import android.annotation.TargetApi
import android.os.Build

fun use(b: Boolean) {}
fun x() = true

@TargetApi(23)
fun n1() {
    if (Build.VERSION.SDK_INT < 24) { when { ((x() && x()) && (Build.VERSION.SDK_INT < 24)) -> { use(x()) }; Build.VERSION.SDK_INT > 20 -> { use(x()) }; else -> { use(Build.VERSION.SDK_INT > 20) } } }
}

@TargetApi(23)
fun n2() {
    if (Build.VERSION.SDK_INT < 24) { when { (x() && (Build.VERSION.SDK_INT < 24 && Build.VERSION.SDK_INT <= 25)) -> { use(x()) }; Build.VERSION.SDK_INT > 20 -> { use(x()) }; else -> { use(Build.VERSION.SDK_INT > 20) } } }
}

@TargetApi(23)
fun n3() {
    if (Build.VERSION.SDK_INT < 24) { when { ((Build.VERSION.SDK_INT < 24 && Build.VERSION.SDK_INT <= 25)) -> { use(x()) }; Build.VERSION.SDK_INT > 20 -> { use(x()) }; else -> { use(Build.VERSION.SDK_INT > 20) } } }
}

@TargetApi(23)
fun n4() {
    if (Build.VERSION.SDK_INT < 24) { when { (Build.VERSION.SDK_INT < 24 && Build.VERSION.SDK_INT <= 25) -> { use(x()) }; Build.VERSION.SDK_INT > 20 -> { use(x()) }; else -> { use(Build.VERSION.SDK_INT > 20) } } }
}

@TargetApi(23)
fun n5() {
    if (Build.VERSION.SDK_INT < 24) { when { (x() && Build.VERSION.SDK_INT < 24) -> { use(x()) }; Build.VERSION.SDK_INT > 20 -> { use(x()) }; else -> { use(Build.VERSION.SDK_INT > 20) } } }
}

@TargetApi(23)
fun n6() {
    if (Build.VERSION.SDK_INT < 24) { when { ((x() && x()) && Build.VERSION.SDK_INT < 24) -> { use(x()) }; Build.VERSION.SDK_INT > 20 -> { use(x()) }; else -> { use(Build.VERSION.SDK_INT > 20) } } }
}

@TargetApi(23)
fun n7() {
    if (Build.VERSION.SDK_INT < 24) { when { (x() && x() && Build.VERSION.SDK_INT < 24) -> { use(x()) }; Build.VERSION.SDK_INT > 20 -> { use(x()) }; else -> { use(Build.VERSION.SDK_INT > 20) } } }
}

@TargetApi(23)
fun n8() {
    if (Build.VERSION.SDK_INT < 24) { when { (Build.VERSION.SDK_INT < 24 && x()) -> { use(x()) }; Build.VERSION.SDK_INT > 20 -> { use(x()) }; else -> { use(Build.VERSION.SDK_INT > 20) } } }
}

@TargetApi(23)
fun n9() {
    if (Build.VERSION.SDK_INT < 24) { when { ((Build.VERSION.SDK_INT < 24 && x()) && x()) -> { use(x()) }; Build.VERSION.SDK_INT > 20 -> { use(x()) }; else -> { use(Build.VERSION.SDK_INT > 20) } } }
}
