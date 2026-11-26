package slack.test;

import com.google.auto.value.AutoValue;

@AutoValue
public abstract class TestClass {
  public static Builder builder() {
    return null;
  }
  @AutoValue.Builder
  public abstract class Builder {
    abstract TestClass build();
  }
}
