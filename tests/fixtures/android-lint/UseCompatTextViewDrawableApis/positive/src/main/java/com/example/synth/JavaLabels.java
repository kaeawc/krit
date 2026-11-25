package com.example.synth;

import android.content.res.ColorStateList;
import android.widget.TextView;

public class JavaLabels {
    void tint(TextView label, ColorStateList colors) {
        label.setCompoundDrawableTintList(colors);
    }
}
