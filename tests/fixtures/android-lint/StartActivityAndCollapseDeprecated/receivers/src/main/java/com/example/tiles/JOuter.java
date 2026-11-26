package com.example.tiles;

import android.content.Intent;
import android.service.quicksettings.TileService;

public class JOuter extends TileService {
    class Inner {
        void go() {
            JOuter.this.startActivityAndCollapse(new Intent());
        }
    }
}
