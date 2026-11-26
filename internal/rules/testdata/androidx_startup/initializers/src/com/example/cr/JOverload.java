package com.example.cr;

import android.content.Context;
import androidx.startup.Initializer;
import java.util.Collections;
import java.util.List;

public class JOverload implements Initializer<Void> {
    @Override
    public Void create(Context context) {
        return null;
    }

    @Override
    public List<Class<? extends Initializer<?>>> dependencies() {
        return Collections.emptyList();
    }

    public List<Class<?>> dependencies(int unused) {
        return Collections.singletonList(OverloadChild.class);
    }
}
