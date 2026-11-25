"""Public rules of the krit module.

    load("@krit//:defs.bzl", "krit_test")

    bazel build //... --aspects=@krit//:defs.bzl%krit_aspect --output_groups=krit
"""

load("//private:krit_aspect.bzl", _KritReportInfo = "KritReportInfo", _krit_aspect = "krit_aspect")
load("//private:krit_test.bzl", _krit_test = "krit_test")

krit_test = _krit_test
krit_aspect = _krit_aspect
KritReportInfo = _KritReportInfo
