"""Public rules of the krit module.

    load("@krit//:defs.bzl", "krit_test")

    bazel build //... --aspects=@krit//:defs.bzl%krit_aspect --output_groups=krit

    bazel run @krit//:export_model -- //...
"""

load("//private:krit_aspect.bzl", _KritReportInfo = "KritReportInfo", _krit_aspect = "krit_aspect")
load("//private:krit_model_aspect.bzl", _krit_model_aspect = "krit_model_aspect")
load("//private:krit_project.bzl", _krit_project_report = "krit_project_report")
load("//private:krit_test.bzl", _krit_test = "krit_test")

krit_test = _krit_test
krit_aspect = _krit_aspect
krit_model_aspect = _krit_model_aspect
krit_project_report = _krit_project_report
KritReportInfo = _KritReportInfo
