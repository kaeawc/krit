package com.example;

import android.view.View;

class Finder {
    View findViewById(int id) { return null; }

    void use() {
        ((android.widget.TextView) findViewById(1)).setText("x");
        Object o = java.util.Objects.requireNonNull(findViewById(2)).getTag();
    }
}
