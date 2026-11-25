package com.example.synth;

import android.content.Context;
import java.util.Objects;

public class JavaHolder {
    private Context context;

    Context getContext() {
        return context;
    }

    Context context() {
        return Objects.requireNonNull(getContext());
    }
}
