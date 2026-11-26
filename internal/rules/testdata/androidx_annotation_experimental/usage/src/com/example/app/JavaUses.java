package com.example.app;

import androidx.annotation.OptIn;
import com.example.api.ApiKt;
import com.example.api.ExperimentalJava;
import com.example.api.ExperimentalKotlin;

class JavaUses {
    void plain() {
        ApiKt.javaMarked();
        ApiKt.kotlinMarked();
        ApiKt.warnMarked();
    }

    @OptIn(markerClass = ExperimentalKotlin.class)
    void optedKotlin() {
        ApiKt.kotlinMarked();
    }

    @ExperimentalJava
    void propagated() {
        ApiKt.javaMarked();
    }
}
