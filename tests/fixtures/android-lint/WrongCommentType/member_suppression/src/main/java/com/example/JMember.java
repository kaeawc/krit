package com.example;

class JMember {
    /* @return one */
    @SuppressWarnings("WrongCommentType")
    public int suppressed() { return 1; }

    /* @return two */
    public int reported() { return 2; }
}
