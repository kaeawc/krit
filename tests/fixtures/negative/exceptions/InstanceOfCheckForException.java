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

  private boolean doSend() throws IOException {
    return true;
  }
}
