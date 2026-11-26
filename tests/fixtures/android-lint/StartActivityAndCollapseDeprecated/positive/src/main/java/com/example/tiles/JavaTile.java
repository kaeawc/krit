package com.example.tiles;

import android.content.Intent;
import android.service.quicksettings.TileService;

public class JavaTile extends TileService {
    @Override
    public void onClick() {
        Intent intent = new Intent(this, SettingsActivity.class);
        startActivityAndCollapse(intent);
        startActivityAndCollapse(new Intent(Intent.ACTION_VIEW));
    }
}
