package com.example.exceptions;

import java.io.IOException;
import java.net.SocketException;

class ErrorClassifier {

  // instanceof type-check on a value that is NOT the caught variable is the
  // "instanceof instead of polymorphism" smell this rule targets.
  String classify(Throwable failure) {
    try {
      return "ok";
    } catch (Exception outer) {
      if (failure instanceof SocketException) {
        return "socket";
      }
      return "other";
    }
  }
}
