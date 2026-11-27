package com.example;

import static android.os.Build.VERSION.SDK_INT;

import android.os.Build;

public class JReview {
    public static void runIf(Runnable r) {
        if (Build.VERSION.SDK_INT >= 31) {
            r.run();
        }
    }

    public static void runIfNoBraces(Runnable r) {
        if (Build.VERSION.SDK_INT >= 31) r.run();
    }

    public static void runEither(Runnable r) {
        if (Build.VERSION.SDK_INT >= 31) {
            r.run();
        } else {
            r.run();
        }
    }

    public static void runOther(Runnable r, Runnable fallback) {
        if (Build.VERSION.SDK_INT >= 31) {
            r.run();
        } else {
            fallback.run();
        }
    }

    public static final boolean ALWAYS = true, SUPPORTED = Build.VERSION.SDK_INT >= 26;

    public boolean shadowed(int SDK_INT) {
        return SDK_INT >= 26;
    }

    public boolean imported() {
        return SDK_INT >= 27;
    }
}
