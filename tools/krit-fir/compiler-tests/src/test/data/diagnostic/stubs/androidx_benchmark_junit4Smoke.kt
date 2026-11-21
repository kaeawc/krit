package stubs

fun benchmarkSmoke(rule: androidx.benchmark.junit4.BenchmarkRule) { rule.measureRepeated { rule.hashCode() } }
