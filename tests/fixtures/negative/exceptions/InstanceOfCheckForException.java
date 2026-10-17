package com.example.exceptions;

import java.io.IOException;
import java.net.SocketException;

class NetworkClient {

  // Narrowing an ALREADY-CAUGHT exception (the catch parameter) with
  // instanceof to decide rethrow-vs-wrap is the idiomatic, legitimate use.
  // Must NOT fire.
  boolean send() throws IOException {
    try {
      return doSend();
    } catch (IOException e) {
      if (e instanceof SocketException) {
        return false;
      }
      throw e;
    }
  }

  // A local unwrapped from the caught variable's cause is still narrowing of
  // the caught exception chain. Must NOT fire.
  void causeLocalUnwrap() {
    try {
      doSend();
    } catch (Exception e) {
      Throwable cause = e.getCause();
      if (cause instanceof IOException) {
        return;
      }
    }
  }

  private boolean doSend() throws IOException {
    return true;
  }
}
