package com.example;

import android.os.Build;

public class JEdge {
    public static boolean commented() {
        // the only statement
        return Build.VERSION.SDK_INT >= 29;
    }
}
