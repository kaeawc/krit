package com.example

import slack.lint.annotations.RestrictCallsTo

interface Listener {
  @RestrictCallsTo
  fun onChanged(value: Int)

  fun notifyAll(value: Int) = onChanged(value)
}
