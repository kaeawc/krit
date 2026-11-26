package com.example

import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.channels.Channel
import kotlinx.coroutines.launch
import kotlinx.coroutines.rx3.rxFlowable
import kotlinx.coroutines.rx3.rxObservable

class Emit {
  fun neverEmits() {
    rxObservable<String> { }
    rxFlowable<String> { println("no emission") }
    rxObservable<String>(Dispatchers.IO) { println("context first") }
  }

  fun emits(channel: Channel<String>) {
    rxObservable<String> { send("a") }
    rxFlowable<String> { trySend("b") }
    rxObservable<String> {
      println("before")
      this.send("c")
    }
    rxObservable<String> { launch { send("d") } }
    rxObservable<String> { this@rxObservable.send("e") }
    // Sending to another channel does not emit.
    rxObservable<String> { channel.send("f") }
  }

  fun nested() {
    rxObservable<String> {
      rxObservable<String> { send("inner") }
    }
    rxFlowable<String> {
      send("outer")
      rxFlowable<String> { println("inner never emits") }
    }
  }
}
