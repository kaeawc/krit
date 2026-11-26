package com.example;

class UseJ {
  void use(Callback cb) {
    ApiKt.configure("x", 1);
    cb.onEvent(1);
  }
}
