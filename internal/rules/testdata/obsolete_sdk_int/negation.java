package p;

import android.os.Build;

class J7 {
    static void use(boolean b) {}
    static boolean x() { return true; }

    static void f0() {
        if (x()) { use(!(Build.VERSION.SDK_INT != 24) || Build.VERSION.SDK_INT <= 20); } else { use((!(Build.VERSION.SDK_INT < 23))); }
    }

    static void f1() {
        use(Build.VERSION.SDK_INT <= 22);
    }

    static void f2() {
        use(Build.VERSION.SDK_INT < 22 || Build.VERSION.SDK_INT == 23 ? !x() : Build.VERSION.SDK_INT >= 20);
    }

    static void f3() {
        if (!x()) { if (!(Build.VERSION.SDK_INT <= 24)) { use(!(Build.VERSION.SDK_INT > 23)); } }
    }

    static void f4() {
        use(Build.VERSION.SDK_INT != 24 && x() ? Build.VERSION.SDK_INT >= 21 : !(Build.VERSION.SDK_INT > 20));
    }

    static void f5() {
        if (!(Build.VERSION.SDK_INT < 24) && !x()) { if (Build.VERSION.SDK_INT <= 22 || !x()) { use(!(Build.VERSION.SDK_INT != 21)); } }
    }

    static void f6() {
        use((!(!(Build.VERSION.SDK_INT != 20))) ? x() && Build.VERSION.SDK_INT == 24 : !(Build.VERSION.SDK_INT > 21) || Build.VERSION.SDK_INT > 21);
    }

    static void f7() {
        if (!(Build.VERSION.SDK_INT != 24)) { use((!(Build.VERSION.SDK_INT < 22))); } else { use(!x()); }
    }

    static void f8() {
        use(!x() && Build.VERSION.SDK_INT != 21);
    }

    static void f9() {
        if ((!(x()))) { if ((!(!x()))) { use(!x()); } }
    }

    static void f10() {
        if (x() && Build.VERSION.SDK_INT >= 25) { use(Build.VERSION.SDK_INT == 24); } else { use(Build.VERSION.SDK_INT == 21 && !x()); }
    }

    static void f11() {
        use(Build.VERSION.SDK_INT != 20 ? Build.VERSION.SDK_INT == 23 && !x() : !(Build.VERSION.SDK_INT >= 24) && !x());
    }

    static void f12() {
        if (Build.VERSION.SDK_INT < 24 && !(Build.VERSION.SDK_INT <= 21)) { if (x() && x()) { use(x()); } }
    }

    static void f13() {
        if (Build.VERSION.SDK_INT <= 25 || x()) { use(!x() && Build.VERSION.SDK_INT >= 20); } else { use(!x()); }
    }

    static void f14() {
        use((!(!x())));
    }

    static void f15() {
        if (!x() || Build.VERSION.SDK_INT > 20) { use(Build.VERSION.SDK_INT < 23); } else { use(!x()); }
    }

    static void f16() {
        if (!(Build.VERSION.SDK_INT == 20) && Build.VERSION.SDK_INT > 23) { use(!(Build.VERSION.SDK_INT != 21) && Build.VERSION.SDK_INT > 25); } else { use(Build.VERSION.SDK_INT < 25 || !x()); }
    }

    static void f17() {
        use(Build.VERSION.SDK_INT >= 23 && Build.VERSION.SDK_INT == 24 ? Build.VERSION.SDK_INT != 25 : Build.VERSION.SDK_INT > 25 && !(Build.VERSION.SDK_INT != 23));
    }

    static void f18() {
        use(!x() && !(Build.VERSION.SDK_INT != 25) ? Build.VERSION.SDK_INT >= 23 : !(Build.VERSION.SDK_INT != 20));
    }

    static void f19() {
        if (Build.VERSION.SDK_INT <= 23 || !x()) { if (x() && x()) { use(Build.VERSION.SDK_INT >= 22); } }
    }

    static void f20() {
        if (!(Build.VERSION.SDK_INT == 22)) { if (x()) { use((!(Build.VERSION.SDK_INT != 23))); } }
    }

    static void f21() {
        if (Build.VERSION.SDK_INT != 22) { use(Build.VERSION.SDK_INT != 23 && Build.VERSION.SDK_INT >= 21); } else { use(Build.VERSION.SDK_INT <= 22 && x()); }
    }

    static void f22() {
        if (x() || x()) { if (!x()) { use(Build.VERSION.SDK_INT > 25); } }
    }

    static void f23() {
        if (!x() || Build.VERSION.SDK_INT <= 24) { if (Build.VERSION.SDK_INT == 23 && !(Build.VERSION.SDK_INT >= 21)) { use(Build.VERSION.SDK_INT != 20); } }
    }

    static void f24() {
        if (Build.VERSION.SDK_INT > 23) { use(Build.VERSION.SDK_INT >= 20); } else { use(!(Build.VERSION.SDK_INT < 25) && !(Build.VERSION.SDK_INT == 21)); }
    }

    static void f25() {
        use(!x() ? !x() || x() : Build.VERSION.SDK_INT != 22);
    }

    static void f26() {
        if (Build.VERSION.SDK_INT > 23 && !x()) { use(Build.VERSION.SDK_INT > 23 && Build.VERSION.SDK_INT < 25); } else { use(!x()); }
    }

    static void f27() {
        use(Build.VERSION.SDK_INT <= 22 || x());
    }

    static void f28() {
        if (!x() && !x()) { use(x() && !x()); } else { use(Build.VERSION.SDK_INT < 20 || Build.VERSION.SDK_INT != 20); }
    }

    static void f29() {
        use(!x());
    }

    static void f30() {
        if (Build.VERSION.SDK_INT != 23) { use(!x()); } else { use(!(Build.VERSION.SDK_INT == 25) && Build.VERSION.SDK_INT < 22); }
    }

    static void f31() {
        use(Build.VERSION.SDK_INT >= 20 ? Build.VERSION.SDK_INT > 23 : (!(!(Build.VERSION.SDK_INT >= 20))));
    }

    static void f32() {
        if (!(Build.VERSION.SDK_INT == 25)) { use(!x() && Build.VERSION.SDK_INT > 24); } else { use(!(Build.VERSION.SDK_INT <= 24)); }
    }

    static void f33() {
        if ((!(x()))) { use(Build.VERSION.SDK_INT < 23); } else { use(Build.VERSION.SDK_INT < 25 && Build.VERSION.SDK_INT > 20); }
    }

    static void f34() {
        if (Build.VERSION.SDK_INT > 20 && !x()) { use(Build.VERSION.SDK_INT < 25 || !x()); } else { use(Build.VERSION.SDK_INT >= 20); }
    }

    static void f35() {
        use(Build.VERSION.SDK_INT > 23 || !(Build.VERSION.SDK_INT <= 24) ? x() : Build.VERSION.SDK_INT > 20);
    }

    static void f36() {
        use(Build.VERSION.SDK_INT <= 23 && !x());
    }

    static void f37() {
        use(Build.VERSION.SDK_INT >= 25);
    }

    static void f38() {
        if (Build.VERSION.SDK_INT > 22) { if (!(Build.VERSION.SDK_INT < 24) && x()) { use(!x()); } }
    }

    static void f39() {
        if (Build.VERSION.SDK_INT < 22 && x()) { use(x()); } else { use(!(Build.VERSION.SDK_INT == 25)); }
    }

    static void f40() {
        if (Build.VERSION.SDK_INT >= 21 && Build.VERSION.SDK_INT >= 20) { use(Build.VERSION.SDK_INT < 20); } else { use(x() && !x()); }
    }

    static void f41() {
        use(Build.VERSION.SDK_INT > 21 && !x() ? !(Build.VERSION.SDK_INT >= 20) && Build.VERSION.SDK_INT > 21 : Build.VERSION.SDK_INT != 25 && Build.VERSION.SDK_INT == 22);
    }

    static void f42() {
        if ((!(!(Build.VERSION.SDK_INT >= 22)))) { if (Build.VERSION.SDK_INT == 22 && Build.VERSION.SDK_INT > 20) { use(!(Build.VERSION.SDK_INT > 25) || !x()); } }
    }

    static void f43() {
        if (!x() && !x()) { use(Build.VERSION.SDK_INT != 20 && !x()); } else { use(!x() && Build.VERSION.SDK_INT < 21); }
    }

    static void f44() {
        use(Build.VERSION.SDK_INT != 21 && x() ? x() : (!(Build.VERSION.SDK_INT < 24)));
    }

    static void f45() {
        use(Build.VERSION.SDK_INT == 24 ? Build.VERSION.SDK_INT >= 24 : !x() || Build.VERSION.SDK_INT >= 20);
    }

    static void f46() {
        if (Build.VERSION.SDK_INT != 20) { use(x() && x()); } else { use(Build.VERSION.SDK_INT != 21 && !x()); }
    }

    static void f47() {
        use(Build.VERSION.SDK_INT != 22);
    }

    static void f48() {
        use(Build.VERSION.SDK_INT == 21);
    }

    static void f49() {
        if ((!(Build.VERSION.SDK_INT <= 20))) { use(Build.VERSION.SDK_INT <= 25); } else { use(Build.VERSION.SDK_INT < 23); }
    }

    static void f50() {
        use(!x());
    }

    static void f51() {
        if (Build.VERSION.SDK_INT >= 20) { use(!(Build.VERSION.SDK_INT != 21)); } else { use(Build.VERSION.SDK_INT == 21 && Build.VERSION.SDK_INT != 22); }
    }

    static void f52() {
        if (!x()) { if (x()) { use(x()); } }
    }

    static void f53() {
        if ((!(!x()))) { if ((!(Build.VERSION.SDK_INT > 23))) { use(Build.VERSION.SDK_INT > 22 && Build.VERSION.SDK_INT <= 23); } }
    }

    static void f54() {
        if (Build.VERSION.SDK_INT < 21) { if (Build.VERSION.SDK_INT != 20 && !x()) { use(Build.VERSION.SDK_INT == 21); } }
    }

    static void f55() {
        use(x() && Build.VERSION.SDK_INT < 23 ? !x() : Build.VERSION.SDK_INT > 25 && Build.VERSION.SDK_INT == 24);
    }

    static void f56() {
        use(!x() && Build.VERSION.SDK_INT < 20);
    }

    static void f57() {
        if (!(Build.VERSION.SDK_INT <= 20) || x()) { if (Build.VERSION.SDK_INT != 20) { use(Build.VERSION.SDK_INT <= 24 || x()); } }
    }

    static void f58() {
        if (!x()) { use(Build.VERSION.SDK_INT < 21 || Build.VERSION.SDK_INT < 22); } else { use(!x()); }
    }

    static void f59() {
        if (Build.VERSION.SDK_INT >= 20) { use(!x()); } else { use(Build.VERSION.SDK_INT <= 25 && Build.VERSION.SDK_INT < 25); }
    }

}
