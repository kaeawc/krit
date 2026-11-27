package com.example;

import android.view.View;
import android.view.ViewGroup;
import androidx.recyclerview.widget.RecyclerView;

public class JSetter extends RecyclerView.Adapter<JSetter.SHolder> {
    static class SHolder extends RecyclerView.ViewHolder {
        int[] slots = new int[1];

        SHolder(View view) {
            super(view);
        }
    }

    @Override
    public SHolder onCreateViewHolder(ViewGroup parent, int viewType) {
        return new SHolder(new View(parent.getContext()));
    }

    @Override
    public int getItemCount() {
        return 10;
    }

    @Override
    public void onBindViewHolder(SHolder holder, int position) {
        holder.slots[0] = position;
    }
}
