package com.example;

import android.view.View;
import android.view.ViewGroup;
import androidx.recyclerview.widget.RecyclerView;

public class JMember extends RecyclerView.Adapter<Holder> {
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
        holder.itemView.setOnClickListener(new View.OnClickListener() {
            int position = 0;

            @Override
            public void onClick(View v) {
                System.out.println(position);
            }
        });
    }
}
