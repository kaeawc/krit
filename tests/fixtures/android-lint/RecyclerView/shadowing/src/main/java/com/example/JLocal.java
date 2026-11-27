package com.example;

import android.view.View;
import android.view.ViewGroup;
import androidx.recyclerview.widget.RecyclerView;

public class JLocal extends RecyclerView.Adapter<Holder> {
    @Override
    public Holder onCreateViewHolder(ViewGroup parent, int viewType) {
        return new Holder(new View(parent.getContext()));
    }

    @Override
    public int getItemCount() {
        return 10;
    }

    @Override
    public void onBindViewHolder(final Holder holder, int position) {
        holder.itemView.setOnClickListener(new View.OnClickListener() {
            @Override
            public void onClick(View v) {
                int position = holder.getAdapterPosition();
                use(position);
            }
        });
    }

    static void use(int p) {}
}
