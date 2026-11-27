package com.example;

import android.content.Context;
import androidx.work.OneTimeWorkRequest;
import androidx.work.WorkManager;

class JReview {
    static class Builder {
        Builder beginWith(int x) {
            return this;
        }

        Builder then(int x) {
            return this;
        }
    }

    void builder(Builder b) {
        Builder t = b.beginWith(1);
        t.then(2);
    }

    JReview(Context context, OneTimeWorkRequest a) {
        WorkManager.getInstance(context).beginWith(a);
    }
}
