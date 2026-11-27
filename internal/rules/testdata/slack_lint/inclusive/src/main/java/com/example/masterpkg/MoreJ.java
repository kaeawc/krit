package com.example.masterpkg;

/** Javadoc */
@SuppressWarnings("unused")
public class MoreJ {
  int a, slaveSecond;
  /** doc */
  private static final int MASTER_CONST = 1;

  /** method doc */
  @Deprecated
  public void masterDoc() {
    try (java.io.StringReader masterReader = new java.io.StringReader("")) {
    } catch (Exception slaveError) {
    }
    for (String slaveItem : new String[0]) { }
  }

  interface SlaveApi { void masterCall(); }
  @interface MasterAnno { int slaveValue(); }
}
