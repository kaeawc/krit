package p

import android.annotation.TargetApi
import android.os.Build
import androidx.annotation.RequiresApi

fun use(b: Boolean) {}
fun x() = true

@RequiresApi(24) fun f0() {
    use(x() || (Build.VERSION.SDK_INT == 22 || Build.VERSION.SDK_INT >= 22))
}

@RequiresApi(19) fun f1() {
    use((Build.VERSION.SDK_INT >= 30 && (!(x()))) || Build.VERSION.SDK_INT != 21)
}

@RequiresApi(24) fun f2() {
    if (Build.VERSION.SDK_INT == 21) { use((Build.VERSION.SDK_INT == 26 && (!(Build.VERSION.SDK_INT >= 20))) && (Build.VERSION.SDK_INT <= 25 && Build.VERSION.SDK_INT == 24)) } else { if (x()) { if (Build.VERSION.SDK_INT >= 26) { use((x() || Build.VERSION.SDK_INT >= 19)) } else { use(Build.VERSION.SDK_INT != 26) } } }
}

@RequiresApi(22) fun f3() {
    if ((Build.VERSION.SDK_INT <= 24 || Build.VERSION.SDK_INT != 25)) { use((x() && Build.VERSION.SDK_INT < 24) || x()) } else { if ((!((Build.VERSION.SDK_INT < 20 && x())))) { use(Build.VERSION.SDK_INT > 20 || ((Build.VERSION.SDK_INT == 19 || Build.VERSION.SDK_INT < 21) || (Build.VERSION.SDK_INT < 20 && Build.VERSION.SDK_INT < 21))) } else { if (Build.VERSION.SDK_INT < 22) { use(((Build.VERSION.SDK_INT > 22 && Build.VERSION.SDK_INT > 22) && (x() && Build.VERSION.SDK_INT < 22))) } else { use((!(Build.VERSION.SDK_INT < 24))) } } }
}

@RequiresApi(24) fun f4() {
    use(((Build.VERSION.SDK_INT == 21 || Build.VERSION.SDK_INT >= 26) && (Build.VERSION.SDK_INT > 21 || Build.VERSION.SDK_INT >= 30)) && Build.VERSION.SDK_INT == 30)
}

@TargetApi(23) fun f5() {
    if (x()) { use(x()) }
}

@RequiresApi(24) fun f6() {
    when { (Build.VERSION.SDK_INT != 21 && Build.VERSION.SDK_INT < 19) -> { use(((Build.VERSION.SDK_INT <= 20 && Build.VERSION.SDK_INT != 26) || (Build.VERSION.SDK_INT < 22 && Build.VERSION.SDK_INT != 21))) }; Build.VERSION.SDK_INT >= 21 -> { use(((!(x())) && (Build.VERSION.SDK_INT < 22 && x()))) }; else -> { use(x()) } }
}

fun f7() {
    if (Build.VERSION.SDK_INT >= 24) { if (x()) { when { ((Build.VERSION.SDK_INT <= 23 && x()) && Build.VERSION.SDK_INT != 30) -> { use(Build.VERSION.SDK_INT == 22) }; x() -> { use(x()) }; else -> { use((!((!(Build.VERSION.SDK_INT > 20))))) } } } else { if (Build.VERSION.SDK_INT < 21) { use(((!(Build.VERSION.SDK_INT == 23)) && (!(x())))) } else { use(x()) } } } else { if ((!(x()))) { if ((Build.VERSION.SDK_INT < 19 && x())) { use(Build.VERSION.SDK_INT > 19) } } }
}

@TargetApi(23) fun f8() {
    if (x()) { if (((Build.VERSION.SDK_INT == 19 && x()) || (Build.VERSION.SDK_INT > 23 && Build.VERSION.SDK_INT >= 20))) { if (((!(Build.VERSION.SDK_INT >= 20)) || Build.VERSION.SDK_INT == 26)) { use(Build.VERSION.SDK_INT < 21) } else { use(((x() || Build.VERSION.SDK_INT != 26) && Build.VERSION.SDK_INT >= 21)) } } } else { use(((x() && x()) && Build.VERSION.SDK_INT < 22)) }
}

@TargetApi(23) fun f9() {
    use(Build.VERSION.SDK_INT != 25)
}

@RequiresApi(19) fun f10() {
    if (Build.VERSION.SDK_INT < 20) { use(Build.VERSION.SDK_INT != 19) } else { use(Build.VERSION.SDK_INT > 19 || (!(x()))) }
}

fun f11() {
    if (Build.VERSION.SDK_INT >= 23) { if ((Build.VERSION.SDK_INT > 25 || x())) { use(Build.VERSION.SDK_INT > 25 || Build.VERSION.SDK_INT <= 20) } else { use(Build.VERSION.SDK_INT <= 22) } } else { use((!(Build.VERSION.SDK_INT == 24))) }
}

@RequiresApi(24) fun f12() {
    use(Build.VERSION.SDK_INT == 20 && (x() && Build.VERSION.SDK_INT <= 30))
}

@RequiresApi(24) fun f13() {
    if (Build.VERSION.SDK_INT <= 20) { if ((!((Build.VERSION.SDK_INT > 26 && Build.VERSION.SDK_INT != 19)))) { if (Build.VERSION.SDK_INT == 30) { use(x()) } else { use(Build.VERSION.SDK_INT == 22) } } else { use(((x() && Build.VERSION.SDK_INT <= 24) || Build.VERSION.SDK_INT >= 19)) } } else { use(((Build.VERSION.SDK_INT > 19 && Build.VERSION.SDK_INT == 22) && (Build.VERSION.SDK_INT < 21 || x()))) }
}

@RequiresApi(22) fun f14() {
    if (Build.VERSION.SDK_INT > 23) { use(Build.VERSION.SDK_INT <= 21 && x()) } else { use((Build.VERSION.SDK_INT < 30 || Build.VERSION.SDK_INT > 30)) }
}

@TargetApi(23) fun f15() {
    use(((Build.VERSION.SDK_INT > 25 && x()) || Build.VERSION.SDK_INT >= 25))
}

@RequiresApi(19) fun f16() {
    use(((x() && Build.VERSION.SDK_INT == 26) || x()) && (!(Build.VERSION.SDK_INT == 26)))
}

fun f17() {
    use(((Build.VERSION.SDK_INT <= 23 || Build.VERSION.SDK_INT > 22) || Build.VERSION.SDK_INT < 21) || Build.VERSION.SDK_INT <= 21)
}

fun f18() {
    use(((!(Build.VERSION.SDK_INT == 30)) || (Build.VERSION.SDK_INT >= 19 || Build.VERSION.SDK_INT > 21)))
}

@TargetApi(23) fun f19() {
    use(Build.VERSION.SDK_INT < 19 && (!((Build.VERSION.SDK_INT <= 25 || Build.VERSION.SDK_INT != 26))))
}

@RequiresApi(24) fun f20() {
    use(Build.VERSION.SDK_INT <= 30 && (!(Build.VERSION.SDK_INT > 20)))
}

@TargetApi(23) fun f21() {
    if ((Build.VERSION.SDK_INT < 22 || (Build.VERSION.SDK_INT >= 25 || x()))) { if ((!(x()))) { use(Build.VERSION.SDK_INT > 23) } else { use(Build.VERSION.SDK_INT == 21 && x()) } }
}

@RequiresApi(24) fun f22() {
    if (Build.VERSION.SDK_INT == 21) { use(((Build.VERSION.SDK_INT < 19 && Build.VERSION.SDK_INT >= 24) || (!(Build.VERSION.SDK_INT <= 23)))) } else { when { Build.VERSION.SDK_INT <= 30 -> { use(Build.VERSION.SDK_INT == 22 && ((Build.VERSION.SDK_INT <= 19 || Build.VERSION.SDK_INT >= 19) || x())) }; ((Build.VERSION.SDK_INT == 23 || x()) && (x() || Build.VERSION.SDK_INT >= 26)) -> { use(((Build.VERSION.SDK_INT <= 25 || Build.VERSION.SDK_INT <= 19) && Build.VERSION.SDK_INT <= 30) && (Build.VERSION.SDK_INT > 26 && Build.VERSION.SDK_INT != 19)) }; else -> { if (Build.VERSION.SDK_INT > 30) { use(Build.VERSION.SDK_INT != 30) } else { use(Build.VERSION.SDK_INT < 22) } } } }
}

@RequiresApi(24) fun f23() {
    use(x() && (x() && x()))
}

@TargetApi(23) fun f24() {
    use((x() || Build.VERSION.SDK_INT <= 20))
}

@RequiresApi(24) fun f25() {
    if (((Build.VERSION.SDK_INT >= 30 && Build.VERSION.SDK_INT > 26) && x())) { use(x()) } else { use(x() && (!((Build.VERSION.SDK_INT == 20 || Build.VERSION.SDK_INT != 23)))) }
}

@RequiresApi(22) fun f26() {
    when { (Build.VERSION.SDK_INT != 24 && (Build.VERSION.SDK_INT > 19 || Build.VERSION.SDK_INT > 24)) -> { use((Build.VERSION.SDK_INT == 22 && Build.VERSION.SDK_INT < 30)) }; Build.VERSION.SDK_INT < 26 -> { if ((Build.VERSION.SDK_INT >= 21 && (!(x())))) { if (x()) { use(((!(Build.VERSION.SDK_INT < 22)) && Build.VERSION.SDK_INT == 20)) } } else { use((Build.VERSION.SDK_INT >= 30 || (!(Build.VERSION.SDK_INT != 20)))) } }; else -> { use(Build.VERSION.SDK_INT > 23 || (Build.VERSION.SDK_INT <= 19 && Build.VERSION.SDK_INT != 19)) } }
}

@RequiresApi(24) fun f27() {
    when { Build.VERSION.SDK_INT > 19 -> { if (Build.VERSION.SDK_INT > 22) { use((!((!(Build.VERSION.SDK_INT == 26))))) } }; Build.VERSION.SDK_INT < 20 -> { if ((!((Build.VERSION.SDK_INT < 22 && x())))) { if (x()) { use(x()) } } else { if (((!(x())) && x())) { use((Build.VERSION.SDK_INT < 22 && (!(x())))) } } }; else -> { use(Build.VERSION.SDK_INT == 19) } }
}

fun f28() {
    use((Build.VERSION.SDK_INT != 20 && Build.VERSION.SDK_INT <= 20) || Build.VERSION.SDK_INT <= 22)
}

@TargetApi(23) fun f29() {
    use((!(x())) || Build.VERSION.SDK_INT < 22)
}

@RequiresApi(24) fun f30() {
    if (x()) { use(((!(Build.VERSION.SDK_INT <= 30)) || x())) }
}

@RequiresApi(24) fun f31() {
    use(((Build.VERSION.SDK_INT <= 30 && Build.VERSION.SDK_INT > 24) || (Build.VERSION.SDK_INT >= 23 || Build.VERSION.SDK_INT <= 26)))
}

@TargetApi(23) fun f32() {
    use(Build.VERSION.SDK_INT != 24 && Build.VERSION.SDK_INT != 19)
}

@RequiresApi(19) fun f33() {
    use(Build.VERSION.SDK_INT > 26 || x())
}

fun f34() {
    if (((Build.VERSION.SDK_INT == 21 && Build.VERSION.SDK_INT >= 21) || Build.VERSION.SDK_INT >= 19)) { use(Build.VERSION.SDK_INT < 30 && (Build.VERSION.SDK_INT != 25 && Build.VERSION.SDK_INT == 24)) } else { if (((!(x())) && x())) { use(x()) } }
}

fun f35() {
    when { ((x() && x()) && (Build.VERSION.SDK_INT >= 24 && Build.VERSION.SDK_INT >= 19)) -> { use(Build.VERSION.SDK_INT >= 26) }; x() -> { use(Build.VERSION.SDK_INT > 24) }; else -> { use((!(Build.VERSION.SDK_INT < 26))) } }
}

fun f36() {
    use(x() && x())
}

@TargetApi(23) fun f37() {
    use(x())
}

@RequiresApi(24) fun f38() {
    if ((x() || (Build.VERSION.SDK_INT > 24 || Build.VERSION.SDK_INT >= 30))) { use(Build.VERSION.SDK_INT > 23 && (!((Build.VERSION.SDK_INT > 23 && Build.VERSION.SDK_INT < 30)))) } else { use((Build.VERSION.SDK_INT >= 30 && Build.VERSION.SDK_INT != 25)) }
}

@TargetApi(23) fun f39() {
    use((Build.VERSION.SDK_INT <= 24 || (!(Build.VERSION.SDK_INT < 22))))
}

@RequiresApi(24) fun f40() {
    when { (!((Build.VERSION.SDK_INT == 23 && Build.VERSION.SDK_INT == 24))) -> { use(Build.VERSION.SDK_INT != 23 || (!((!(Build.VERSION.SDK_INT > 20))))) }; Build.VERSION.SDK_INT < 20 -> { if (x()) { use((!((Build.VERSION.SDK_INT < 25 && Build.VERSION.SDK_INT == 26))) && ((Build.VERSION.SDK_INT > 26 && x()) && (Build.VERSION.SDK_INT == 21 || Build.VERSION.SDK_INT == 23))) } else { use(Build.VERSION.SDK_INT <= 20) } }; else -> { use((x() && (Build.VERSION.SDK_INT < 23 && Build.VERSION.SDK_INT == 22)) && Build.VERSION.SDK_INT == 25) } }
}

@RequiresApi(24) fun f41() {
    if (Build.VERSION.SDK_INT != 25) { if (((!(Build.VERSION.SDK_INT < 24)) || Build.VERSION.SDK_INT < 24)) { if ((!(Build.VERSION.SDK_INT < 19))) { use(Build.VERSION.SDK_INT <= 20) } } } else { use(Build.VERSION.SDK_INT < 22) }
}

fun f42() {
    use(((x() || x()) && (x() || Build.VERSION.SDK_INT > 21)))
}

@RequiresApi(22) fun f43() {
    if ((Build.VERSION.SDK_INT >= 24 || (x() && Build.VERSION.SDK_INT < 24))) { use((!(x()))) }
}

fun f44() {
    use(x() && x())
}

@RequiresApi(19) fun f45() {
    if (((Build.VERSION.SDK_INT > 21 && Build.VERSION.SDK_INT < 24) && (Build.VERSION.SDK_INT == 25 && Build.VERSION.SDK_INT != 24))) { use((Build.VERSION.SDK_INT != 25 || (Build.VERSION.SDK_INT <= 25 || Build.VERSION.SDK_INT == 20))) }
}

@RequiresApi(19) fun f46() {
    use((Build.VERSION.SDK_INT <= 24 && (!(Build.VERSION.SDK_INT >= 26))) || x())
}

@RequiresApi(22) fun f47() {
    use(Build.VERSION.SDK_INT >= 24)
}

@RequiresApi(19) fun f48() {
    if ((Build.VERSION.SDK_INT >= 22 && (Build.VERSION.SDK_INT != 22 && x()))) { if ((Build.VERSION.SDK_INT > 30 && (Build.VERSION.SDK_INT != 21 || Build.VERSION.SDK_INT > 30))) { if (((Build.VERSION.SDK_INT < 30 || Build.VERSION.SDK_INT > 24) && Build.VERSION.SDK_INT != 26)) { use(x()) } else { use(((!(Build.VERSION.SDK_INT != 24)) && x())) } } } else { use((Build.VERSION.SDK_INT <= 22 || (Build.VERSION.SDK_INT <= 22 && Build.VERSION.SDK_INT >= 19))) }
}

@RequiresApi(22) fun f49() {
    if (((Build.VERSION.SDK_INT != 25 && Build.VERSION.SDK_INT != 23) && Build.VERSION.SDK_INT <= 25)) { use(Build.VERSION.SDK_INT < 19 && ((!(Build.VERSION.SDK_INT < 19)) || (Build.VERSION.SDK_INT > 23 || Build.VERSION.SDK_INT >= 21))) } else { if (Build.VERSION.SDK_INT < 23) { if (Build.VERSION.SDK_INT > 30) { use(((Build.VERSION.SDK_INT >= 22 && Build.VERSION.SDK_INT >= 19) && Build.VERSION.SDK_INT == 25)) } else { use((Build.VERSION.SDK_INT >= 20 || Build.VERSION.SDK_INT > 30)) } } }
}

fun f50() {
    use(((!(Build.VERSION.SDK_INT < 26)) || (Build.VERSION.SDK_INT >= 26 && Build.VERSION.SDK_INT == 24)) && ((!(Build.VERSION.SDK_INT != 25)) && x()))
}

@RequiresApi(19) fun f51() {
    use(Build.VERSION.SDK_INT != 21)
}

@TargetApi(23) fun f52() {
    use(x())
}

fun f53() {
    use(x() && ((!(Build.VERSION.SDK_INT < 24)) || Build.VERSION.SDK_INT > 22))
}

fun f54() {
    use(((!(Build.VERSION.SDK_INT <= 21)) && (!(Build.VERSION.SDK_INT == 19))))
}

@RequiresApi(24) fun f55() {
    use((!(Build.VERSION.SDK_INT == 23)))
}

@TargetApi(23) fun f56() {
    if ((!(Build.VERSION.SDK_INT > 19))) { if ((x() || x())) { use(Build.VERSION.SDK_INT <= 30) } } else { use((!((Build.VERSION.SDK_INT > 22 && Build.VERSION.SDK_INT != 20)))) }
}

@RequiresApi(24) fun f57() {
    use(Build.VERSION.SDK_INT >= 21 && Build.VERSION.SDK_INT < 22)
}

@RequiresApi(24) fun f58() {
    use((Build.VERSION.SDK_INT != 21 && Build.VERSION.SDK_INT >= 19))
}

fun f59() {
    use(Build.VERSION.SDK_INT > 23)
}
