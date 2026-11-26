package com.example;

import android.os.Build;

public class JVersions {
    public static final boolean IS_N = Build.VERSION.SDK_INT >= Build.VERSION_CODES.N;

    public static boolean isAtLeastM() {
        return Build.VERSION.SDK_INT >= 23;
    }
}
