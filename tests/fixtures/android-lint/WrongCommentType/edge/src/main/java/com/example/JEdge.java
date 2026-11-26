package com.example;

/* @author someone */
public class JEdge {
    /* @Deprecated upper
     * @return lower */
    public int upperThenLower() { return 1; }

    /* Mentions {@linkplain JEdge the edge}.
     * @param x x */
    public void linkplain(int x) {}

    /* @param x the value. With a period */
    public void period(int x) {}

    /* text with an @inline at mid line */
    public void midLine() {}

    /* @return abstract */
    public JEdge() {}

    enum Kind {
        /* @deprecated constant */
        A
    }

    interface Api {
        /* @return interface method */
        int call();

        /* @since 1 */
        int CONSTANT = 1;
    }
}
