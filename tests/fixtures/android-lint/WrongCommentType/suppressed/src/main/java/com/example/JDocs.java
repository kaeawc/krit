package com.example;

@SuppressWarnings("WrongCommentType")
public class JDocs {
    /* Returns {@link JDocs} instances.
     * @return the docs */
    public JDocs create() { return this; }

    /*
     * @param x the value
     */
    void set(int x) {}

    /* {@inheritDoc} */
    @Override
    public String toString() { return ""; }

    /* @deprecated use something else */
    int field;
}
