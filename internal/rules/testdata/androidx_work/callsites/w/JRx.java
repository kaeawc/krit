package com.example;

import android.content.Context;
import androidx.annotation.NonNull;
import androidx.work.Data;
import androidx.work.RxWorker;
import androidx.work.WorkerParameters;
import io.reactivex.Single;

class JRx extends RxWorker {
    JRx(@NonNull Context context, @NonNull WorkerParameters params) {
        super(context, params);
    }

    @NonNull
    @Override
    public Single<Result> createWork() {
        setProgress(Data.EMPTY);
        return Single.just(Result.success());
    }

    static class Plain {
        void setProgress(Data data) {}

        void run() {
            setProgress(Data.EMPTY);
        }
    }
}
