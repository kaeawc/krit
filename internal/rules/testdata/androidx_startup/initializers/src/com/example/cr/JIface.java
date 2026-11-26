package com.example.cr;

import androidx.startup.Initializer;
import java.util.Collections;
import java.util.List;

public interface JIface extends Initializer<Void> {
    @Override
    default List<Class<? extends Initializer<?>>> dependencies() {
        return Collections.singletonList(IfaceChild.class);
    }
}
