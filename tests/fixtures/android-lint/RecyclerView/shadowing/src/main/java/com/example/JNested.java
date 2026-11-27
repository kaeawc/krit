package com.example;

import android.view.View;
import android.view.ViewGroup;
import androidx.recyclerview.widget.RecyclerView;

public class JNested extends RecyclerView.Adapter<Holder> {
    @Override
    public Holder onCreateViewHolder(ViewGroup parent, int viewType) {
        return new Holder(new View(parent.getContext()));
    }

    @Override
    public int getItemCount() {
        return 10;
    }

    @Override
    public void onBindViewHolder(Holder holder, int position) {
        if (position > 0) {
            int c;
            c = position;
            holder.itemView.setOnClickListener(new View.OnClickListener() {
                @Override
                public void onClick(View v) {
                    System.out.println(c);
                }
            });
        }
    }
}
