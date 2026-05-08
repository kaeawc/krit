package com.example;

import android.os.Handler;
import android.os.Message;

class MyActivity {
    class MyHandler extends Handler {
        @Override
        public void handleMessage(Message msg) {
            // handle
        }
    }

    Object anonymous = new Handler() {
        @Override
        public void handleMessage(Message msg) {
            // handle
        }
    };
}
