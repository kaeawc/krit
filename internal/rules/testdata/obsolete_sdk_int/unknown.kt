package p

import android.os.Build

fun use(b: Boolean) {}
fun x() = true

// CASE ge||x
fun t1() {
    if ((Build.VERSION.SDK_INT >= 23) || x()) {
        use(Build.VERSION.SDK_INT >= 22)
        use(Build.VERSION.SDK_INT < 23)
    } else {
        use(Build.VERSION.SDK_INT >= 22)
        use(Build.VERSION.SDK_INT < 23)
    }
}

// CASE x||ge
fun t2() {
    if (x() || (Build.VERSION.SDK_INT >= 23)) {
        use(Build.VERSION.SDK_INT >= 22)
        use(Build.VERSION.SDK_INT < 23)
    } else {
        use(Build.VERSION.SDK_INT >= 22)
        use(Build.VERSION.SDK_INT < 23)
    }
}

// CASE ge&&x
fun t3() {
    if ((Build.VERSION.SDK_INT >= 23) && x()) {
        use(Build.VERSION.SDK_INT >= 22)
        use(Build.VERSION.SDK_INT < 23)
    } else {
        use(Build.VERSION.SDK_INT >= 22)
        use(Build.VERSION.SDK_INT < 23)
    }
}

// CASE x&&ge
fun t4() {
    if (x() && (Build.VERSION.SDK_INT >= 23)) {
        use(Build.VERSION.SDK_INT >= 22)
        use(Build.VERSION.SDK_INT < 23)
    } else {
        use(Build.VERSION.SDK_INT >= 22)
        use(Build.VERSION.SDK_INT < 23)
    }
}

// CASE x&&(ge||x)
fun t5() {
    if (x() && ((Build.VERSION.SDK_INT >= 23) || x())) {
        use(Build.VERSION.SDK_INT >= 22)
        use(Build.VERSION.SDK_INT < 23)
    } else {
        use(Build.VERSION.SDK_INT >= 22)
        use(Build.VERSION.SDK_INT < 23)
    }
}

// CASE (ge||x)&&x
fun t6() {
    if (((Build.VERSION.SDK_INT >= 23) || x()) && x()) {
        use(Build.VERSION.SDK_INT >= 22)
        use(Build.VERSION.SDK_INT < 23)
    } else {
        use(Build.VERSION.SDK_INT >= 22)
        use(Build.VERSION.SDK_INT < 23)
    }
}

// CASE x||(ge&&x)
fun t7() {
    if (x() || ((Build.VERSION.SDK_INT >= 23) && x())) {
        use(Build.VERSION.SDK_INT >= 22)
        use(Build.VERSION.SDK_INT < 23)
    } else {
        use(Build.VERSION.SDK_INT >= 22)
        use(Build.VERSION.SDK_INT < 23)
    }
}

// CASE (x&&ge)||x
fun t8() {
    if ((x() && (Build.VERSION.SDK_INT >= 23)) || x()) {
        use(Build.VERSION.SDK_INT >= 22)
        use(Build.VERSION.SDK_INT < 23)
    } else {
        use(Build.VERSION.SDK_INT >= 22)
        use(Build.VERSION.SDK_INT < 23)
    }
}

// CASE ge||lt
fun t9() {
    if ((Build.VERSION.SDK_INT >= 23) || (Build.VERSION.SDK_INT < 23)) {
        use(Build.VERSION.SDK_INT >= 22)
        use(Build.VERSION.SDK_INT < 23)
    } else {
        use(Build.VERSION.SDK_INT >= 22)
        use(Build.VERSION.SDK_INT < 23)
    }
}

// CASE lt||ge
fun t10() {
    if ((Build.VERSION.SDK_INT < 23) || (Build.VERSION.SDK_INT >= 23)) {
        use(Build.VERSION.SDK_INT >= 22)
        use(Build.VERSION.SDK_INT < 23)
    } else {
        use(Build.VERSION.SDK_INT >= 22)
        use(Build.VERSION.SDK_INT < 23)
    }
}

// CASE ge&&lt
fun t11() {
    if ((Build.VERSION.SDK_INT >= 23) && (Build.VERSION.SDK_INT < 23)) {
        use(Build.VERSION.SDK_INT >= 22)
        use(Build.VERSION.SDK_INT < 23)
    } else {
        use(Build.VERSION.SDK_INT >= 22)
        use(Build.VERSION.SDK_INT < 23)
    }
}

// CASE ge||ge2
fun t12() {
    if ((Build.VERSION.SDK_INT >= 23) || (Build.VERSION.SDK_INT >= 24)) {
        use(Build.VERSION.SDK_INT >= 22)
        use(Build.VERSION.SDK_INT < 23)
    } else {
        use(Build.VERSION.SDK_INT >= 22)
        use(Build.VERSION.SDK_INT < 23)
    }
}

// CASE ge&&ge2
fun t13() {
    if ((Build.VERSION.SDK_INT >= 23) && (Build.VERSION.SDK_INT >= 24)) {
        use(Build.VERSION.SDK_INT >= 22)
        use(Build.VERSION.SDK_INT < 23)
    } else {
        use(Build.VERSION.SDK_INT >= 22)
        use(Build.VERSION.SDK_INT < 23)
    }
}

// CASE !(ge&&x)
fun t14() {
    if (!((Build.VERSION.SDK_INT >= 23) && x())) {
        use(Build.VERSION.SDK_INT >= 22)
        use(Build.VERSION.SDK_INT < 23)
    } else {
        use(Build.VERSION.SDK_INT >= 22)
        use(Build.VERSION.SDK_INT < 23)
    }
}

// CASE !(ge||x)
fun t15() {
    if (!((Build.VERSION.SDK_INT >= 23) || x())) {
        use(Build.VERSION.SDK_INT >= 22)
        use(Build.VERSION.SDK_INT < 23)
    } else {
        use(Build.VERSION.SDK_INT >= 22)
        use(Build.VERSION.SDK_INT < 23)
    }
}

// CASE !(x||ge)
fun t16() {
    if (!(x() || (Build.VERSION.SDK_INT >= 23))) {
        use(Build.VERSION.SDK_INT >= 22)
        use(Build.VERSION.SDK_INT < 23)
    } else {
        use(Build.VERSION.SDK_INT >= 22)
        use(Build.VERSION.SDK_INT < 23)
    }
}

// CASE !(lt||x)
fun t17() {
    if (!((Build.VERSION.SDK_INT < 23) || x())) {
        use(Build.VERSION.SDK_INT >= 22)
        use(Build.VERSION.SDK_INT < 23)
    } else {
        use(Build.VERSION.SDK_INT >= 22)
        use(Build.VERSION.SDK_INT < 23)
    }
}

// CASE !(lt&&x)
fun t18() {
    if (!((Build.VERSION.SDK_INT < 23) && x())) {
        use(Build.VERSION.SDK_INT >= 22)
        use(Build.VERSION.SDK_INT < 23)
    } else {
        use(Build.VERSION.SDK_INT >= 22)
        use(Build.VERSION.SDK_INT < 23)
    }
}

// CASE !x
fun t19() {
    if (!x()) {
        use(Build.VERSION.SDK_INT >= 22)
        use(Build.VERSION.SDK_INT < 23)
    } else {
        use(Build.VERSION.SDK_INT >= 22)
        use(Build.VERSION.SDK_INT < 23)
    }
}

// CASE !(!ge)
fun t20() {
    if (!(!(Build.VERSION.SDK_INT >= 23))) {
        use(Build.VERSION.SDK_INT >= 22)
        use(Build.VERSION.SDK_INT < 23)
    } else {
        use(Build.VERSION.SDK_INT >= 22)
        use(Build.VERSION.SDK_INT < 23)
    }
}

// CASE ge&&x&&x
fun t21() {
    if ((Build.VERSION.SDK_INT >= 23) && x() && x()) {
        use(Build.VERSION.SDK_INT >= 22)
        use(Build.VERSION.SDK_INT < 23)
    } else {
        use(Build.VERSION.SDK_INT >= 22)
        use(Build.VERSION.SDK_INT < 23)
    }
}

// CASE x&&x&&ge
fun t22() {
    if (x() && x() && (Build.VERSION.SDK_INT >= 23)) {
        use(Build.VERSION.SDK_INT >= 22)
        use(Build.VERSION.SDK_INT < 23)
    } else {
        use(Build.VERSION.SDK_INT >= 22)
        use(Build.VERSION.SDK_INT < 23)
    }
}

// CASE ge||x||x
fun t23() {
    if ((Build.VERSION.SDK_INT >= 23) || x() || x()) {
        use(Build.VERSION.SDK_INT >= 22)
        use(Build.VERSION.SDK_INT < 23)
    } else {
        use(Build.VERSION.SDK_INT >= 22)
        use(Build.VERSION.SDK_INT < 23)
    }
}

// CASE x||x||ge
fun t24() {
    if (x() || x() || (Build.VERSION.SDK_INT >= 23)) {
        use(Build.VERSION.SDK_INT >= 22)
        use(Build.VERSION.SDK_INT < 23)
    } else {
        use(Build.VERSION.SDK_INT >= 22)
        use(Build.VERSION.SDK_INT < 23)
    }
}

// CASE lt||x
fun t25() {
    if ((Build.VERSION.SDK_INT < 23) || x()) {
        use(Build.VERSION.SDK_INT >= 22)
        use(Build.VERSION.SDK_INT < 23)
    } else {
        use(Build.VERSION.SDK_INT >= 22)
        use(Build.VERSION.SDK_INT < 23)
    }
}

// CASE x||lt
fun t26() {
    if (x() || (Build.VERSION.SDK_INT < 23)) {
        use(Build.VERSION.SDK_INT >= 22)
        use(Build.VERSION.SDK_INT < 23)
    } else {
        use(Build.VERSION.SDK_INT >= 22)
        use(Build.VERSION.SDK_INT < 23)
    }
}
