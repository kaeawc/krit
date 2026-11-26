package com.example.lookalike;

class JLookalike {
    static class Builder {
        Builder setRequiresDeviceIdle(boolean value) { return this; }
        Builder setRequiresCharging(boolean value) { return this; }
    }

    void local() {
        Builder builder = new Builder();
        builder.setRequiresDeviceIdle(true);
        builder.setRequiresCharging(true);
    }
}
