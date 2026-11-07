package com.example.exceptions;

import java.io.IOException;
import java.net.SocketException;

class ErrorClassifier {

  // instanceof on the caught variable is what multiple catch blocks are for.
  String classify() {
    try {
      return "ok";
    } catch (Exception outer) {
      if (outer instanceof SocketException) {
        return "socket";
      }
      return "other";
    }
  }
}
