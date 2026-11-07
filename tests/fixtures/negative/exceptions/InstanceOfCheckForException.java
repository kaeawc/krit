package com.example.exceptions;

import java.io.IOException;
import java.net.SocketException;

class NetworkClient {

  // A value other than the caught variable, such as its cause, is not reported.
  void causeLocalUnwrap(Throwable failure) {
    try {
      doSend();
    } catch (Exception e) {
      Throwable cause = e.getCause();
      if (cause instanceof IOException || failure instanceof SocketException) {
        return;
      }
    }
  }

  private boolean doSend() throws IOException {
    return true;
  }
}
