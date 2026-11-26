package com.example;

import androidx.annotation.NonNull;
import androidx.work.Configuration;

public class JConfig implements Configuration.Provider {
    @NonNull
    @Override
    public Configuration getWorkManagerConfiguration() {
        return new Configuration.Builder().build();
    }
}
