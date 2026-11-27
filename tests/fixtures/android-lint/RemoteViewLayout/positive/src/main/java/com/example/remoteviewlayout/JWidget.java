package com.example.remoteviewlayout;

import android.content.Context;
import android.widget.RemoteViews;

class JWidget {
    RemoteViews views(Context context) {
        return new RemoteViews(context.getPackageName(), R.layout.widget);
    }
}
