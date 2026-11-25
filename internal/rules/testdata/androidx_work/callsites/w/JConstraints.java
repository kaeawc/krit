package com.example;

import androidx.work.Constraints;

class JConstraints {
    void sameVariable() {
        Constraints.Builder builder = new Constraints.Builder();
        builder.setRequiresDeviceIdle(true);
        builder.setRequiresCharging(true);
    }

    Constraints chained() {
        return new Constraints.Builder()
            .setRequiresDeviceIdle(true)
            .setRequiresCharging(true)
            .build();
    }

    void differentBuilders() {
        Constraints.Builder idle = new Constraints.Builder();
        Constraints.Builder charging = new Constraints.Builder();
        idle.setRequiresDeviceIdle(true);
        charging.setRequiresCharging(true);
    }

    static final boolean CHARGING = true;

    void constantArgument() {
        Constraints.Builder builder = new Constraints.Builder();
        builder.setRequiresDeviceIdle(true);
        builder.setRequiresCharging(CHARGING);
    }
}
