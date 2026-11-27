package com.example;

public class JBind {
    int x = 1; // trailing
    /* @see JBind */
    void afterTrailingLine() {}

    int y = 2; /* @return trailing block */
    void afterTrailingBlock() {}

    int w = 4; /* plain */ /* @return same line as code */
    void afterTwoTrailing() {}

    int e = 5; /* spans
        two lines */ /* @see JBind */
    void afterMultiLineTrailing() {}
}
