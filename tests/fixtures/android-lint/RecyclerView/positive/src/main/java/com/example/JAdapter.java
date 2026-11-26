package com.example;

import android.view.View;
import android.view.ViewGroup;
import androidx.recyclerview.widget.RecyclerView;
import java.util.List;

public class JAdapter extends RecyclerView.Adapter<Holder> {
    private int saved;

    @Override
    public Holder onCreateViewHolder(ViewGroup parent, int viewType) { return new Holder(new View(parent.getContext())); }

    @Override
    public int getItemCount() { return 10; }

    @Override
    public void onBindViewHolder(Holder viewHolder, int position) {
        int copy;
        copy = position;
        saved = copy;
    }

    @Override
    public void onBindViewHolder(Holder viewHolder, final int pos, List<Object> payloads) {
        viewHolder.itemView.setOnClickListener(new View.OnClickListener() {
            @Override
            public void onClick(View v) {
                System.out.println(pos);
            }
        });
    }

    static class Sub extends JAdapter {
        @Override
        public void onBindViewHolder(Holder h, int p) {
            Runnable r = new Runnable() { public void run() { System.out.println(p); } };
        }
    }
}
