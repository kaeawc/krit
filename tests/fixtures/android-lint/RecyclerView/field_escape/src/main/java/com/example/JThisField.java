package com.example;

import android.view.View;
import android.view.ViewGroup;
import androidx.recyclerview.widget.RecyclerView;

// `this.f` is the field even where a local f is in scope.
public class JThisField extends RecyclerView.Adapter<JThisField.JHolder> {
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
    public void onBindViewHolder(JHolder holder, int position) {
        int f = 0;
        this.f = position;
        use(f);
    }

    static void use(int p) {}
}
