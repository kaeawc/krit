package p;

import android.os.Build;
import static android.os.Build.VERSION.SDK_INT;
import static android.os.Build.VERSION_CODES.KITKAT;

class J6 {
    static final int S19 = 19;
    final int inst19 = 19;
    int mut19 = 19;
    boolean x() { return true; }
    void a() {
        if (SDK_INT >= 19) {}
        if (SDK_INT >= KITKAT) {}
        if (Build.VERSION.SDK_INT >= S19) {}
        if (Build.VERSION.SDK_INT >= inst19) {}
        if (Build.VERSION.SDK_INT >= mut19) {}
        final int loc = 19;
        if (Build.VERSION.SDK_INT >= loc) {}
        int loc2 = 19;
        if (Build.VERSION.SDK_INT >= loc2) {}
        if (Build.VERSION.SDK_INT >= (19)) {}
        if (Build.VERSION.SDK_INT >= 19 + 1) {}
        if (Build.VERSION.SDK_INT >= 0x13) {}
        if (Build.VERSION.SDK_INT >= 19L) {}
        if ((Build.VERSION.SDK_INT) >= 19) {}
        if (android.os.Build.VERSION.SDK_INT >= 19) {}
        boolean b = Build.VERSION.SDK_INT >= 19 ? x() : false;
        while (Build.VERSION.SDK_INT >= 19) { break; }
        for (; Build.VERSION.SDK_INT >= 19;) { break; }
        do { } while (Build.VERSION.SDK_INT >= 19);
        if (Build.VERSION.SDK_INT >= 23) { if (Build.VERSION.SDK_INT >= 22) {} }
        if (Build.VERSION.SDK_INT >= 23) { } else { if (Build.VERSION.SDK_INT >= 22) {} }
        if (Build.VERSION.SDK_INT < 23) { } else { if (Build.VERSION.SDK_INT >= 22) {} }
        if (Build.VERSION.SDK_INT >= 23 && Build.VERSION.SDK_INT >= 22) {}
        if (Build.VERSION.SDK_INT < 23 || Build.VERSION.SDK_INT >= 23) {}
        boolean c = Build.VERSION.SDK_INT >= 23 ? Build.VERSION.SDK_INT >= 22 : Build.VERSION.SDK_INT >= 22;
        Runnable r = () -> { if (Build.VERSION.SDK_INT >= 19) {} };
        if (Build.VERSION.SDK_INT >= 23) { Runnable r2 = () -> { if (Build.VERSION.SDK_INT >= 22) {} }; }
        if (Build.VERSION.SDK_INT >= 23) { new Object() { void m() { if (Build.VERSION.SDK_INT >= 22) {} } }; }
        switch (1) { case 1: if (Build.VERSION.SDK_INT >= 19) {} break; }
        if (Build.VERSION.SDK_INT >= 23) return;
        if (Build.VERSION.SDK_INT >= 22) {}
        if (Build.VERSION.SDK_INT >= 19 == x()) {}
        if (Build.VERSION.SDK_INT >= 19 == true) {}
        if (!(Build.VERSION.SDK_INT >= 19)) {}
        if (Build.VERSION.SDK_INT >= 19 | x()) {}
        if (Build.VERSION.SDK_INT >= 19 & x()) {}
        if (Build.VERSION.SDK_INT >= 19 ^ x()) {}
    }
}
