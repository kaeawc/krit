package com.example;

import android.content.Context;
import androidx.annotation.NonNull;
import androidx.work.Worker;
import androidx.work.WorkerParameters;

class JPackageWorker extends Worker {
    JPackageWorker(@NonNull Context context, @NonNull WorkerParameters params) { super(context, params); }

    @NonNull
    @Override
    public Result doWork() { return Result.success(); }

    private static class JPrivateNested extends Worker {
        JPrivateNested(@NonNull Context context, @NonNull WorkerParameters params) { super(context, params); }

        @NonNull
        @Override
        public Result doWork() { return Result.success(); }
    }

    public static class JPublicNested extends Worker {
        public JPublicNested(@NonNull Context context, @NonNull WorkerParameters params) { super(context, params); }

        @NonNull
        @Override
        public Result doWork() { return Result.success(); }
    }
}
