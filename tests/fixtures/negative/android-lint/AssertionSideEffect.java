package com.example;

import java.util.List;

class Counter {
    private int count;

    void check(List<Integer> values) {
        // Not an assert statement: a test helper named like one.
        assertTrue(count++ < 10);
        count++;
        // Lint does not follow Java calls, so a mutating call in an assertion is not reported.
        assert values.add(1);
        assert count > 0 : "count is " + count;
    }

    static void assertTrue(boolean condition) {}
}
