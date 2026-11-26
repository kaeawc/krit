package com.example.masterpkg;

class WhitelistJ {
  int masterField, other;
  WhitelistJ() {}
  void slaveMethod(int masterParam) {
    int slaveLocal = 0;
    masterLabel:
    for (int masterI = 0; masterI < 1; masterI++) { break masterLabel; }
    Runnable r = () -> {};
    java.util.function.IntConsumer c = slaveArg -> {};
  }
  enum State { SLAVE }
}
