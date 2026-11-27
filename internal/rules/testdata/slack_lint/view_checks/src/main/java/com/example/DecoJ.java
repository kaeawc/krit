package com.example;

import android.content.Context;
import android.graphics.Canvas;
import android.view.LayoutInflater;
import androidx.recyclerview.widget.RecyclerView;
import com.example.databinding.ItemBinding;

class DecoJ extends RecyclerView.ItemDecoration {
  private final LayoutInflater inflater;

  DecoJ(LayoutInflater inflater) { this.inflater = inflater; }

  @Override
  public void onDraw(Canvas c, RecyclerView parent) {
    inflater.inflate(1, null);
    ItemBinding.inflate(inflater);
    LayoutInflater.from(parent.getContext()).inflate(1, parent, false);
  }

  void strings(MyView m, android.view.View v) {
    m.toString();
    v.toString();
    String s = "" + m;
  }
}
