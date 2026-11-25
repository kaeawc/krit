package com.example.synth;

import android.content.Context;
import android.content.res.ColorStateList;

public class JavaTheming {
    ColorStateList tint(Context context) {
        return context.getResources().getColorStateList(android.R.color.white);
    }
}
