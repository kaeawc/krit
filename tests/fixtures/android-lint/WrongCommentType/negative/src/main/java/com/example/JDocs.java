package com.example;

public class JDocs {
    /** @return real javadoc */
    public int real() { return 1; }

    /* @Override looks like an annotation */
    public void annotation() {}

    /* {@code x} is not one of the inline tags */
    public void code() {}

    /* @return private */
    private int hidden() { return 1; }

    /* (non-Javadoc)
     * @see Object#toString()
     */
    public String describe() { return ""; }
}
