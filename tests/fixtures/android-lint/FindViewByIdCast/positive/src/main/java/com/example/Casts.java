package com.example;

import android.app.Activity;
import android.view.View;
import java.util.Objects;

public class Casts extends Activity {
    static <T> T identity(T value) { return value; }

    void bind(View root) {
        Objects.requireNonNull(findViewById(R.id.title)).setVisibility(View.GONE);
        identity(root.findViewById(R.id.title)).setVisibility(View.GONE);
        Objects.requireNonNull(requireViewById(R.id.title)).getId();
        identity(findViewById(R.id.title)).toString().length();
    }
}
