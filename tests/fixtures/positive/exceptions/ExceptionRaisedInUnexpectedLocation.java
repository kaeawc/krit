package test;

class Value {
  @Override
  public boolean equals(Object other) {
    throw new IllegalStateException("comparison failed");
  }
}
