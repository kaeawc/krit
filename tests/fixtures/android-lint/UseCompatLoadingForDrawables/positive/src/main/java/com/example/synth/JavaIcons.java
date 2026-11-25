package com.example.synth;

import android.content.Context;
import android.graphics.drawable.Drawable;

public class JavaIcons {
    Drawable icon(Context context) {
        return context.getDrawable(android.R.drawable.ic_delete);
    }

    Drawable fromResources(Context context) {
        return context.getResources().getDrawable(android.R.drawable.ic_delete);
    }
}
