package com.example.tiles;

import android.annotation.SuppressLint;
import android.content.Intent;
import android.service.quicksettings.TileService;

public class JavaTile extends TileService {
    @SuppressLint("StartActivityAndCollapseDeprecated")
    @Override
    public void onClick() {
        Intent intent = new Intent(this, SettingsActivity.class);
        startActivityAndCollapse(intent);
        startActivityAndCollapse(new Intent(Intent.ACTION_VIEW));
    }
}
