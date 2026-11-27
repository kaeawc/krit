package com.example;

import androidx.annotation.ReturnThis;

class JBase {
    @ReturnThis
    JBase self() { return this; }
}

class JChild extends JBase {
    @Override
    JBase self() { return new JBase(); }
}
