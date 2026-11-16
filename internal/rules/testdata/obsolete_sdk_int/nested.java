package p;

import android.annotation.TargetApi;
import android.os.Build;
import androidx.annotation.RequiresApi;

class J7 {
    static void use(boolean b) {}
    static boolean x() { return true; }

    @RequiresApi(24) static void f0() {
        use(Build.VERSION.SDK_INT >= 22);
    }

    @RequiresApi(19) static void f1() {
        use((x() && (Build.VERSION.SDK_INT <= 30 || Build.VERSION.SDK_INT == 26)));
    }

    @RequiresApi(24) static void f2() {
        if (x()) { use(Build.VERSION.SDK_INT == 24); } else { use(((Build.VERSION.SDK_INT >= 26 && Build.VERSION.SDK_INT != 30) && Build.VERSION.SDK_INT >= 26) || (x() && x())); }
    }

    @RequiresApi(22) static void f3() {
        if (x()) { use(x() ? (!(Build.VERSION.SDK_INT <= 21)) : ((Build.VERSION.SDK_INT > 30 && x()) && (x() || x()))); } else { use(Build.VERSION.SDK_INT >= 23 || Build.VERSION.SDK_INT == 24); }
    }

    @RequiresApi(24) static void f4() {
        if ((Build.VERSION.SDK_INT < 30 && (!(Build.VERSION.SDK_INT <= 20)))) { use((!(x())) ? x() : x()); }
    }

    @TargetApi(23) static void f5() {
        use(x());
    }

    @RequiresApi(24) static void f6() {
        if (((Build.VERSION.SDK_INT < 22 || x()) && (Build.VERSION.SDK_INT <= 20 || Build.VERSION.SDK_INT >= 22))) { if ((!((!(Build.VERSION.SDK_INT == 25))))) { use(x() || (!(x()))); } else { if (Build.VERSION.SDK_INT != 22) { use(Build.VERSION.SDK_INT > 30); } } } else { use((Build.VERSION.SDK_INT == 21 || Build.VERSION.SDK_INT == 25) ? ((Build.VERSION.SDK_INT == 19 || Build.VERSION.SDK_INT != 22) || x()) : ((!(Build.VERSION.SDK_INT >= 19)) && (Build.VERSION.SDK_INT >= 26 && Build.VERSION.SDK_INT == 30))); }
    }

    static void f7() {
        use(Build.VERSION.SDK_INT >= 25);
    }

    @TargetApi(23) static void f8() {
        use((x() || Build.VERSION.SDK_INT <= 20) || Build.VERSION.SDK_INT < 22);
    }

    @TargetApi(23) static void f9() {
        use(((x() || Build.VERSION.SDK_INT > 22) || Build.VERSION.SDK_INT < 20));
    }

    @RequiresApi(19) static void f10() {
        use((!(x())) || (!((Build.VERSION.SDK_INT <= 24 || Build.VERSION.SDK_INT >= 19))));
    }

    static void f11() {
        use(((!(Build.VERSION.SDK_INT == 20)) && x()) ? Build.VERSION.SDK_INT <= 23 : (!((Build.VERSION.SDK_INT >= 23 || Build.VERSION.SDK_INT == 22))));
    }

    @RequiresApi(24) static void f12() {
        if ((!(Build.VERSION.SDK_INT != 20))) { use(x()); } else { use((Build.VERSION.SDK_INT != 26 && x())); }
    }

    @RequiresApi(24) static void f13() {
        use((Build.VERSION.SDK_INT != 21 && Build.VERSION.SDK_INT < 23) ? x() : ((!(x())) || (Build.VERSION.SDK_INT <= 25 || x())));
    }

    @RequiresApi(22) static void f14() {
        use((!((!(x())))));
    }

    @TargetApi(23) static void f15() {
        use(Build.VERSION.SDK_INT > 22 || (!((Build.VERSION.SDK_INT != 19 || Build.VERSION.SDK_INT <= 26))));
    }

    @RequiresApi(19) static void f16() {
        use((x() || Build.VERSION.SDK_INT <= 30) && (Build.VERSION.SDK_INT >= 24 && (x() || Build.VERSION.SDK_INT > 19)));
    }

    static void f17() {
        if ((x() || (Build.VERSION.SDK_INT >= 21 || Build.VERSION.SDK_INT <= 23))) { use(Build.VERSION.SDK_INT == 30 || Build.VERSION.SDK_INT > 21); } else { if ((x() && (Build.VERSION.SDK_INT == 22 && Build.VERSION.SDK_INT > 22))) { use(Build.VERSION.SDK_INT <= 21 ? (Build.VERSION.SDK_INT >= 24 || (!(Build.VERSION.SDK_INT != 20))) : Build.VERSION.SDK_INT <= 24); } else { if ((Build.VERSION.SDK_INT != 25 || Build.VERSION.SDK_INT >= 21)) { use((Build.VERSION.SDK_INT <= 22 && Build.VERSION.SDK_INT >= 23)); } } }
    }

    static void f18() {
        use((Build.VERSION.SDK_INT > 21 || Build.VERSION.SDK_INT != 30));
    }

    @TargetApi(23) static void f19() {
        use((!(Build.VERSION.SDK_INT >= 24)));
    }

    @RequiresApi(24) static void f20() {
        use(Build.VERSION.SDK_INT > 22 && (!((Build.VERSION.SDK_INT < 22 || Build.VERSION.SDK_INT != 30))));
    }

    @TargetApi(23) static void f21() {
        use(Build.VERSION.SDK_INT != 20);
    }

    @RequiresApi(24) static void f22() {
        use((!((Build.VERSION.SDK_INT > 24 || Build.VERSION.SDK_INT != 20))) ? Build.VERSION.SDK_INT > 25 : x());
    }

    @RequiresApi(24) static void f23() {
        if (((Build.VERSION.SDK_INT <= 26 || Build.VERSION.SDK_INT != 21) || (Build.VERSION.SDK_INT > 23 || Build.VERSION.SDK_INT <= 21))) { use(((Build.VERSION.SDK_INT > 23 && Build.VERSION.SDK_INT != 22) && (!(Build.VERSION.SDK_INT != 25))) ? ((Build.VERSION.SDK_INT != 22 && Build.VERSION.SDK_INT <= 25) || Build.VERSION.SDK_INT >= 25) : Build.VERSION.SDK_INT != 21); }
    }

    @TargetApi(23) static void f24() {
        use(Build.VERSION.SDK_INT <= 19 ? Build.VERSION.SDK_INT < 21 : (Build.VERSION.SDK_INT != 19 && x()));
    }

    @RequiresApi(24) static void f25() {
        if (Build.VERSION.SDK_INT != 20) { use(Build.VERSION.SDK_INT == 19); }
    }

    @RequiresApi(22) static void f26() {
        if ((Build.VERSION.SDK_INT > 25 && Build.VERSION.SDK_INT > 26)) { if (Build.VERSION.SDK_INT > 19) { use((Build.VERSION.SDK_INT <= 26 || Build.VERSION.SDK_INT == 21) && Build.VERSION.SDK_INT >= 20); } }
    }

    @RequiresApi(24) static void f27() {
        use(Build.VERSION.SDK_INT != 26 && x());
    }

    static void f28() {
        use((!((!(Build.VERSION.SDK_INT >= 21)))) && (!(Build.VERSION.SDK_INT < 21)));
    }

    @TargetApi(23) static void f29() {
        if ((!((!(x()))))) { use(x() ? Build.VERSION.SDK_INT == 21 : x()); } else { if ((!((Build.VERSION.SDK_INT < 20 && x())))) { use(Build.VERSION.SDK_INT < 24); } else { use(x() && Build.VERSION.SDK_INT > 30); } }
    }

    @RequiresApi(24) static void f30() {
        use((Build.VERSION.SDK_INT >= 20 && (Build.VERSION.SDK_INT >= 30 && Build.VERSION.SDK_INT != 26)) ? Build.VERSION.SDK_INT <= 21 : (!(x())));
    }

    @RequiresApi(24) static void f31() {
        if (((Build.VERSION.SDK_INT >= 24 && x()) && Build.VERSION.SDK_INT == 24)) { if (Build.VERSION.SDK_INT == 22) { if ((x() && (Build.VERSION.SDK_INT != 26 && Build.VERSION.SDK_INT != 21))) { use((!(x()))); } else { use(Build.VERSION.SDK_INT >= 19); } } else { use(Build.VERSION.SDK_INT == 23); } }
    }

    @TargetApi(23) static void f32() {
        if (x()) { if ((x() || x())) { use(Build.VERSION.SDK_INT >= 22); } } else { use(x()); }
    }

    @RequiresApi(19) static void f33() {
        if (Build.VERSION.SDK_INT >= 19) { if ((x() && (Build.VERSION.SDK_INT >= 26 && Build.VERSION.SDK_INT != 21))) { if (Build.VERSION.SDK_INT >= 22) { use(Build.VERSION.SDK_INT <= 22); } else { use(Build.VERSION.SDK_INT <= 30); } } else { if (((Build.VERSION.SDK_INT == 25 && Build.VERSION.SDK_INT >= 30) && Build.VERSION.SDK_INT != 21)) { use(((!(Build.VERSION.SDK_INT > 22)) && Build.VERSION.SDK_INT < 23)); } else { use(x()); } } }
    }

    static void f34() {
        use(Build.VERSION.SDK_INT <= 25);
    }

    static void f35() {
        use(Build.VERSION.SDK_INT != 22 || Build.VERSION.SDK_INT != 26);
    }

    static void f36() {
        use(Build.VERSION.SDK_INT >= 24 || (Build.VERSION.SDK_INT >= 20 && Build.VERSION.SDK_INT == 22));
    }

    @TargetApi(23) static void f37() {
        use((x() && (Build.VERSION.SDK_INT > 30 || Build.VERSION.SDK_INT > 21)));
    }

    @RequiresApi(24) static void f38() {
        use(((Build.VERSION.SDK_INT >= 22 || Build.VERSION.SDK_INT <= 23) || Build.VERSION.SDK_INT <= 21) || (!(Build.VERSION.SDK_INT != 24)));
    }

    @TargetApi(23) static void f39() {
        use(x() ? x() : Build.VERSION.SDK_INT >= 30);
    }

    @RequiresApi(24) static void f40() {
        use(Build.VERSION.SDK_INT == 26 ? (x() || (Build.VERSION.SDK_INT >= 19 && Build.VERSION.SDK_INT >= 23)) : x());
    }

    @RequiresApi(24) static void f41() {
        use((!(Build.VERSION.SDK_INT > 23)) || ((x() && Build.VERSION.SDK_INT <= 20) && (!(Build.VERSION.SDK_INT > 24))));
    }

    static void f42() {
        if (x()) { use(((Build.VERSION.SDK_INT == 24 && Build.VERSION.SDK_INT == 21) || (!(Build.VERSION.SDK_INT >= 24)))); } else { use(Build.VERSION.SDK_INT < 26 ? (Build.VERSION.SDK_INT == 19 && (Build.VERSION.SDK_INT != 20 || Build.VERSION.SDK_INT == 25)) : x()); }
    }

    @RequiresApi(22) static void f43() {
        use(Build.VERSION.SDK_INT == 26);
    }

    static void f44() {
        use((!((Build.VERSION.SDK_INT < 30 && Build.VERSION.SDK_INT < 26))) || Build.VERSION.SDK_INT > 26);
    }

    @RequiresApi(19) static void f45() {
        use(((Build.VERSION.SDK_INT < 21 && x()) || (!(Build.VERSION.SDK_INT == 23))) && (!((Build.VERSION.SDK_INT < 30 && Build.VERSION.SDK_INT >= 22))));
    }

    @RequiresApi(19) static void f46() {
        use(((x() || x()) && Build.VERSION.SDK_INT < 30) && (Build.VERSION.SDK_INT > 19 && Build.VERSION.SDK_INT == 24));
    }

    @RequiresApi(22) static void f47() {
        use(Build.VERSION.SDK_INT != 19 || (Build.VERSION.SDK_INT < 21 && Build.VERSION.SDK_INT >= 20));
    }

    @RequiresApi(19) static void f48() {
        use(Build.VERSION.SDK_INT < 25 ? (Build.VERSION.SDK_INT > 21 && Build.VERSION.SDK_INT < 21) : (Build.VERSION.SDK_INT <= 22 || Build.VERSION.SDK_INT == 26));
    }

    @RequiresApi(22) static void f49() {
        use(Build.VERSION.SDK_INT <= 21 || (!((Build.VERSION.SDK_INT < 25 && Build.VERSION.SDK_INT != 20))));
    }

    static void f50() {
        use(Build.VERSION.SDK_INT < 20 || ((Build.VERSION.SDK_INT < 23 && Build.VERSION.SDK_INT != 24) && (x() && Build.VERSION.SDK_INT == 24)));
    }

    @RequiresApi(19) static void f51() {
        use((Build.VERSION.SDK_INT != 19 && Build.VERSION.SDK_INT <= 20));
    }

    @TargetApi(23) static void f52() {
        if (((!(Build.VERSION.SDK_INT <= 20)) && (x() && Build.VERSION.SDK_INT < 24))) { use((x() && Build.VERSION.SDK_INT != 20) && Build.VERSION.SDK_INT > 30); } else { use((Build.VERSION.SDK_INT > 24 && (Build.VERSION.SDK_INT == 22 && Build.VERSION.SDK_INT == 21))); }
    }

    static void f53() {
        use((!((x() && x()))) || Build.VERSION.SDK_INT <= 26);
    }

    static void f54() {
        use((x() || (Build.VERSION.SDK_INT <= 20 && Build.VERSION.SDK_INT == 19)) ? x() : ((Build.VERSION.SDK_INT != 24 && x()) || (Build.VERSION.SDK_INT <= 19 && x())));
    }

    @RequiresApi(24) static void f55() {
        use(Build.VERSION.SDK_INT < 30);
    }

    @TargetApi(23) static void f56() {
        use(((Build.VERSION.SDK_INT != 23 || Build.VERSION.SDK_INT > 21) && (Build.VERSION.SDK_INT <= 20 || x())) || x());
    }

    @RequiresApi(24) static void f57() {
        use(((Build.VERSION.SDK_INT < 24 || x()) || (!(x()))) && (!(x())));
    }

    @RequiresApi(24) static void f58() {
        use(Build.VERSION.SDK_INT != 30 && (Build.VERSION.SDK_INT <= 20 && Build.VERSION.SDK_INT < 30));
    }

    static void f59() {
        use(((!(Build.VERSION.SDK_INT != 23)) && Build.VERSION.SDK_INT <= 26));
    }

}
