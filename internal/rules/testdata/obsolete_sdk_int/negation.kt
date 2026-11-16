package p

import android.os.Build

fun use(b: Boolean) {}
fun x() = true

fun f0() {
    if (Build.VERSION.SDK_INT >= 24) { use(Build.VERSION.SDK_INT == 21) } else { use(x()) }
}

fun f1() {
    if (x()) { use(Build.VERSION.SDK_INT != 21 || Build.VERSION.SDK_INT >= 24) } else { use(!(Build.VERSION.SDK_INT != 24)) }
}

fun f2() {
    if (!(Build.VERSION.SDK_INT == 22) || Build.VERSION.SDK_INT != 23) { use(Build.VERSION.SDK_INT >= 24) } else { use(!x() && !x()) }
}

fun f3() {
    when { Build.VERSION.SDK_INT != 23 -> { use(x()) }; Build.VERSION.SDK_INT > 24 && !x() -> { use(!(Build.VERSION.SDK_INT <= 23) || x()) }; else -> { use(Build.VERSION.SDK_INT == 22 && Build.VERSION.SDK_INT != 23) } }
}

fun f4() {
    use(!x() && Build.VERSION.SDK_INT != 25)
}

fun f5() {
    if (!x()) { use(!x()) } else { use(!x()) }
}

fun f6() {
    use(Build.VERSION.SDK_INT < 21)
}

fun f7() {
    use(!x())
}

fun f8() {
    use(Build.VERSION.SDK_INT == 20)
}

fun f9() {
    use(Build.VERSION.SDK_INT < 20)
}

fun f10() {
    if (Build.VERSION.SDK_INT <= 25 || !x()) { if (Build.VERSION.SDK_INT != 25 && Build.VERSION.SDK_INT > 23) { use((!(Build.VERSION.SDK_INT >= 22))) } }
}

fun f11() {
    use(!x())
}

fun f12() {
    if (x()) { if ((!(!x()))) { use(Build.VERSION.SDK_INT <= 24) } }
}

fun f13() {
    if (Build.VERSION.SDK_INT < 21 || !x()) { use(!x() && !x()) } else { use(Build.VERSION.SDK_INT == 23 && !x()) }
}

fun f14() {
    when { !x() -> { use(Build.VERSION.SDK_INT < 23 && Build.VERSION.SDK_INT == 23) }; !x() || x() -> { use(Build.VERSION.SDK_INT != 20) }; else -> { use(Build.VERSION.SDK_INT >= 22 || Build.VERSION.SDK_INT < 24) } }
}

fun f15() {
    when { (!(!(Build.VERSION.SDK_INT > 21))) -> { use((!(Build.VERSION.SDK_INT > 22))) }; !(Build.VERSION.SDK_INT >= 22) && !x() -> { use(Build.VERSION.SDK_INT > 23 && Build.VERSION.SDK_INT <= 25) }; else -> { use(x() && Build.VERSION.SDK_INT > 22) } }
}

fun f16() {
    if (!(Build.VERSION.SDK_INT > 25) || Build.VERSION.SDK_INT > 22) { if (Build.VERSION.SDK_INT == 22 && Build.VERSION.SDK_INT < 21) { use(!x()) } }
}

fun f17() {
    when { Build.VERSION.SDK_INT == 25 && Build.VERSION.SDK_INT == 21 -> { use((!(Build.VERSION.SDK_INT >= 25))) }; Build.VERSION.SDK_INT != 25 && !x() -> { use(x()) }; else -> { use(Build.VERSION.SDK_INT == 20) } }
}

fun f18() {
    use(Build.VERSION.SDK_INT != 22)
}

fun f19() {
    when { Build.VERSION.SDK_INT <= 23 && !x() -> { use(Build.VERSION.SDK_INT <= 20 || !x()) }; Build.VERSION.SDK_INT <= 21 && Build.VERSION.SDK_INT >= 24 -> { use(Build.VERSION.SDK_INT < 22) }; else -> { use(Build.VERSION.SDK_INT == 22) } }
}

fun f20() {
    when { Build.VERSION.SDK_INT != 20 || !(Build.VERSION.SDK_INT < 20) -> { use(Build.VERSION.SDK_INT == 20) }; x() -> { use(!x()) }; else -> { use(Build.VERSION.SDK_INT < 25) } }
}

fun f21() {
    if ((!(Build.VERSION.SDK_INT >= 25))) { if (!x() && Build.VERSION.SDK_INT == 21) { use(x()) } }
}

fun f22() {
    if (!x()) { use(!x()) } else { use(!(Build.VERSION.SDK_INT == 21)) }
}

fun f23() {
    use(!(Build.VERSION.SDK_INT < 21) && Build.VERSION.SDK_INT <= 22)
}

fun f24() {
    if (Build.VERSION.SDK_INT <= 20 || !x()) { if (Build.VERSION.SDK_INT != 25 || !x()) { use(Build.VERSION.SDK_INT != 20) } }
}

fun f25() {
    use(x())
}

fun f26() {
    when { x() -> { use(!(Build.VERSION.SDK_INT == 20)) }; Build.VERSION.SDK_INT < 21 -> { use(!(Build.VERSION.SDK_INT < 20)) }; else -> { use((!(!x()))) } }
}

fun f27() {
    if (Build.VERSION.SDK_INT == 25) { use(Build.VERSION.SDK_INT < 22 && !x()) } else { use(!x()) }
}

fun f28() {
    if (Build.VERSION.SDK_INT > 22) { if (x() && Build.VERSION.SDK_INT != 23) { use(Build.VERSION.SDK_INT <= 20) } }
}

fun f29() {
    if (Build.VERSION.SDK_INT <= 23) { use(Build.VERSION.SDK_INT > 23 || Build.VERSION.SDK_INT != 22) } else { use(Build.VERSION.SDK_INT < 22 || Build.VERSION.SDK_INT == 20) }
}

fun f30() {
    use(Build.VERSION.SDK_INT > 23 && Build.VERSION.SDK_INT < 24)
}

fun f31() {
    if (!(Build.VERSION.SDK_INT < 21)) { use(!(Build.VERSION.SDK_INT < 24) && !x()) } else { use(!(Build.VERSION.SDK_INT == 21)) }
}

fun f32() {
    if ((!(Build.VERSION.SDK_INT < 24))) { use(!(Build.VERSION.SDK_INT != 20) || Build.VERSION.SDK_INT == 22) } else { use(x()) }
}

fun f33() {
    use(Build.VERSION.SDK_INT > 25 && Build.VERSION.SDK_INT <= 25)
}

fun f34() {
    use(!x())
}

fun f35() {
    if (Build.VERSION.SDK_INT < 21 || !x()) { use((!(Build.VERSION.SDK_INT != 23))) } else { use(Build.VERSION.SDK_INT != 21) }
}

fun f36() {
    if (Build.VERSION.SDK_INT < 20 || Build.VERSION.SDK_INT != 22) { if (!(Build.VERSION.SDK_INT <= 25) && Build.VERSION.SDK_INT > 20) { use(Build.VERSION.SDK_INT <= 21) } }
}

fun f37() {
    use(x())
}

fun f38() {
    if (Build.VERSION.SDK_INT <= 25 || !x()) { use(Build.VERSION.SDK_INT >= 21 || Build.VERSION.SDK_INT < 21) } else { use(!(Build.VERSION.SDK_INT < 24) && !x()) }
}

fun f39() {
    if ((!(Build.VERSION.SDK_INT > 22))) { use(Build.VERSION.SDK_INT != 22) } else { use((!(Build.VERSION.SDK_INT != 20))) }
}

fun f40() {
    if (Build.VERSION.SDK_INT < 25) { if ((!(Build.VERSION.SDK_INT < 25))) { use(!x()) } }
}

fun f41() {
    if (!x() || Build.VERSION.SDK_INT <= 20) { use(!x()) } else { use(Build.VERSION.SDK_INT >= 20 && Build.VERSION.SDK_INT < 24) }
}

fun f42() {
    if (Build.VERSION.SDK_INT <= 25 && x()) { use(!x() && !x()) } else { use((!(Build.VERSION.SDK_INT >= 22))) }
}

fun f43() {
    if (x()) { if (Build.VERSION.SDK_INT != 25 && Build.VERSION.SDK_INT <= 24) { use(x() && Build.VERSION.SDK_INT < 21) } }
}

fun f44() {
    if (!x() || Build.VERSION.SDK_INT <= 20) { if (!(Build.VERSION.SDK_INT >= 20)) { use((!(Build.VERSION.SDK_INT >= 25))) } }
}

fun f45() {
    use(!x() || x())
}

fun f46() {
    if (Build.VERSION.SDK_INT > 23) { use(Build.VERSION.SDK_INT == 21 && !(Build.VERSION.SDK_INT != 21)) } else { use(Build.VERSION.SDK_INT >= 23 && Build.VERSION.SDK_INT >= 25) }
}

fun f47() {
    use(!x() && Build.VERSION.SDK_INT <= 25)
}

fun f48() {
    use(Build.VERSION.SDK_INT != 21)
}

fun f49() {
    when { (!(Build.VERSION.SDK_INT < 21)) -> { use(Build.VERSION.SDK_INT < 25) }; (!(Build.VERSION.SDK_INT > 23)) -> { use(!x()) }; else -> { use(!(Build.VERSION.SDK_INT < 21)) } }
}

fun f50() {
    if (Build.VERSION.SDK_INT >= 25) { use(Build.VERSION.SDK_INT != 24) } else { use(Build.VERSION.SDK_INT != 25 && Build.VERSION.SDK_INT != 25) }
}

fun f51() {
    if (Build.VERSION.SDK_INT <= 21 && Build.VERSION.SDK_INT != 21) { use(x()) } else { use(x() && Build.VERSION.SDK_INT > 25) }
}

fun f52() {
    use(!(Build.VERSION.SDK_INT != 21) && Build.VERSION.SDK_INT >= 25)
}

fun f53() {
    use(!(Build.VERSION.SDK_INT > 22) && Build.VERSION.SDK_INT == 21)
}

fun f54() {
    if (Build.VERSION.SDK_INT <= 22 || !(Build.VERSION.SDK_INT != 23)) { if (Build.VERSION.SDK_INT <= 20 && Build.VERSION.SDK_INT < 21) { use(Build.VERSION.SDK_INT == 25 && Build.VERSION.SDK_INT > 22) } }
}

fun f55() {
    if (Build.VERSION.SDK_INT > 21) { if ((!(!x()))) { use(Build.VERSION.SDK_INT != 24 || Build.VERSION.SDK_INT > 23) } }
}

fun f56() {
    if (Build.VERSION.SDK_INT <= 25) { if (Build.VERSION.SDK_INT < 23 && Build.VERSION.SDK_INT >= 21) { use(!x() && Build.VERSION.SDK_INT < 20) } }
}

fun f57() {
    if (Build.VERSION.SDK_INT == 24) { if (x() || Build.VERSION.SDK_INT < 22) { use(Build.VERSION.SDK_INT < 24) } }
}

fun f58() {
    if (Build.VERSION.SDK_INT <= 24) { use(Build.VERSION.SDK_INT == 24 || !x()) } else { use(Build.VERSION.SDK_INT >= 21 && Build.VERSION.SDK_INT != 21) }
}

fun f59() {
    if (Build.VERSION.SDK_INT <= 24 && Build.VERSION.SDK_INT <= 24) { use(!x() && Build.VERSION.SDK_INT != 21) } else { use(!x() && !(Build.VERSION.SDK_INT > 25)) }
}
