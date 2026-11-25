package com.example;

class Counter {
    private int count;

    void check(int[] values) {
        assert count++ < 10 : "too many checks";
        int seen = 0;
        assert (seen = values.length) > 0;
    }
}
