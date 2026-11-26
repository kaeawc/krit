package com.example;

import android.app.Activity;
import android.view.View;
import java.util.Objects;

public class Casts extends Activity {
    static View viewOnly(View value) { return value; }
    static <T extends View> T bounded(T value) { return value; }
    static <T> void consume(T value) {}

    void bind(View root) {
        View v = Objects.requireNonNull(findViewById(R.id.title));
        viewOnly(findViewById(R.id.title)).setVisibility(View.GONE);
        bounded(findViewById(R.id.title)).setVisibility(View.GONE);
        consume(findViewById(R.id.title));
        ((View) Objects.requireNonNull(findViewById(R.id.title))).setVisibility(View.GONE);
        Objects.requireNonNull((View) findViewById(R.id.title)).setVisibility(View.GONE);
    }
}
