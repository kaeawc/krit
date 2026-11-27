package com.example;

import android.view.View;
import android.view.ViewGroup;
import androidx.recyclerview.widget.RecyclerView;

public class JFields extends RecyclerView.Adapter<JFields.JHolder> {
    static class JHolder extends RecyclerView.ViewHolder {
        int pos;

        JHolder(View view) {
            super(view);
        }
    }

    @Override
    public JHolder onCreateViewHolder(ViewGroup parent, int viewType) {
        return new JHolder(new View(parent.getContext()));
    }

    @Override
    public int getItemCount() {
        return 10;
    }

    @Override
    public void onBindViewHolder(JHolder holder, int position) {
        holder.pos = position;
    }
}
