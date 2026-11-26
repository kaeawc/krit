package com.example.tiles;

import android.app.PendingIntent;
import android.content.Intent;
import android.service.quicksettings.TileService;

public class JavaTile extends TileService {
    @Override
    public void onClick() {
        PendingIntent pending = PendingIntent.getActivity(this, 0, new Intent(), PendingIntent.FLAG_IMMUTABLE);
        startActivityAndCollapse(pending);
    }

    static class Lookalike {
        void startActivityAndCollapse(Intent intent) {}

        void run() {
            startActivityAndCollapse(new Intent());
        }
    }
}
