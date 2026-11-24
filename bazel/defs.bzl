"""Public rules of the krit module.

    load("@krit//:defs.bzl", "krit_test")
"""

load("//private:krit_test.bzl", _krit_test = "krit_test")

krit_test = _krit_test
