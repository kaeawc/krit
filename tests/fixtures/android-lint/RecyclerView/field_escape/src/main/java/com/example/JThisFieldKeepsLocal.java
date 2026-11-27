package com.example;

import android.view.View;
import android.view.ViewGroup;
import androidx.recyclerview.widget.RecyclerView;

// Writing the field `this.f` leaves the local f, a copy of the position, holding it.
public class JThisFieldKeepsLocal extends RecyclerView.Adapter<JThisFieldKeepsLocal.JHolder> {
    static class JHolder extends RecyclerView.ViewHolder {
        JHolder(View view) {
            super(view);
        }
    }

    int f;

    @Override
    public JHolder onCreateViewHolder(ViewGroup parent, int viewType) {
        return new JHolder(new View(parent.getContext()));
    }

    @Override
    public int getItemCount() {
        return 10;
    }

    @Override
    public void onBindViewHolder(final JHolder holder, int position) {
        final int f;
        f = position;
        this.f = 0;
        holder.itemView.setOnClickListener(new View.OnClickListener() {
            @Override
            public void onClick(View v) {
                use(f);
            }
        });
    }

    static void use(int p) {}
}
