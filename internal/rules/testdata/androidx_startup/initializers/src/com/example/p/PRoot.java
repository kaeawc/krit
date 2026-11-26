package com.example.p;

import android.content.Context;
import androidx.startup.Initializer;
import com.example.q.*;
import java.util.Collections;
import java.util.List;

public class PRoot implements Initializer<Void> {
    @Override
    public Void create(Context context) {
        return null;
    }

    @Override
    public List<Class<? extends Initializer<?>>> dependencies() {
        return Collections.singletonList(Child.class);
    }
}
