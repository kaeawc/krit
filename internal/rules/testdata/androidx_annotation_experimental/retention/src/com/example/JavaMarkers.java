package com.example;

import androidx.annotation.RequiresOptIn;
import java.lang.annotation.Retention;
import java.lang.annotation.RetentionPolicy;

@RequiresOptIn
@interface JavaDefault {}

@RequiresOptIn
@Retention(RetentionPolicy.CLASS)
@interface JavaClass {}

@RequiresOptIn
@Retention(RetentionPolicy.RUNTIME)
@interface JavaRuntime {}

@RequiresOptIn(level = RequiresOptIn.Level.ERROR)
@Retention(RetentionPolicy.SOURCE)
@interface JavaSource {}
