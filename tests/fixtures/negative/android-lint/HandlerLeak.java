package com.example;

import android.os.Handler;
import android.os.Looper;
import android.os.Message;

class MyActivity {
    static class StaticHandler extends Handler {
        @Override
        public void handleMessage(Message msg) {
            // handle safely
        }
    }

    class LooperHandler extends Handler {
        LooperHandler(Looper looper) {
            super(looper);
        }

        @Override
        public void handleMessage(Message msg) {
            // handle safely
        }
    }
}
