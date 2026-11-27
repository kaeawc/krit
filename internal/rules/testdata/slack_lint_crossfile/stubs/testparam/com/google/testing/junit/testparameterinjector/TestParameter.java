package com.google.testing.junit.testparameterinjector;
import java.lang.annotation.*;
@Retention(RetentionPolicy.RUNTIME) @Target({ElementType.FIELD, ElementType.PARAMETER})
public @interface TestParameter { String[] value() default {}; }
