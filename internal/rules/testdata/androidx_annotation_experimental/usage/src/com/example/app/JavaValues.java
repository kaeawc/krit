package com.example.app;

import com.example.api.MarkedClass;

class JavaValues {
    MarkedClass field;

    void local() {
        MarkedClass c = new MarkedClass();
        c.member();
    }

    void param(MarkedClass p) {
        p.member();
    }

    void viaField() {
        field.member();
    }

    void shadowedByLocal(MarkedClass field) {
        Object other = field;
        String text = "";
        text.length();
    }
}
