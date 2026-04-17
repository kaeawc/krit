package com.example.circuit

import androidx.compose.runtime.Composable

// A @Composable function with PascalCase name - should NOT be flagged by FunctionNaming
// when ignoreAnnotated includes 'Composable'
@Composable
fun MyScreenContent() {
    val x = 1
    val y = 2
    println("composable")
}

// A long method with 100 lines - under Circuit's 120 threshold, should NOT be flagged
fun longMethodUnderCircuitThreshold() {
    val a1 = "line"
    val a2 = "line"
    val a3 = "line"
    val a4 = "line"
    val a5 = "line"
    val a6 = "line"
    val a7 = "line"
    val a8 = "line"
    val a9 = "line"
    val a10 = "line"
    val a11 = "line"
    val a12 = "line"
    val a13 = "line"
    val a14 = "line"
    val a15 = "line"
    val a16 = "line"
    val a17 = "line"
    val a18 = "line"
    val a19 = "line"
    val a20 = "line"
    val a21 = "line"
    val a22 = "line"
    val a23 = "line"
    val a24 = "line"
    val a25 = "line"
    val a26 = "line"
    val a27 = "line"
    val a28 = "line"
    val a29 = "line"
    val a30 = "line"
    val a31 = "line"
    val a32 = "line"
    val a33 = "line"
    val a34 = "line"
    val a35 = "line"
    val a36 = "line"
    val a37 = "line"
    val a38 = "line"
    val a39 = "line"
    val a40 = "line"
    val a41 = "line"
    val a42 = "line"
    val a43 = "line"
    val a44 = "line"
    val a45 = "line"
    val a46 = "line"
    val a47 = "line"
    val a48 = "line"
    val a49 = "line"
    val a50 = "line"
    val a51 = "line"
    val a52 = "line"
    val a53 = "line"
    val a54 = "line"
    val a55 = "line"
    val a56 = "line"
    val a57 = "line"
    val a58 = "line"
    val a59 = "line"
    val a60 = "line"
    val a61 = "line"
    val a62 = "line"
    val a63 = "line"
    val a64 = "line"
    val a65 = "line"
    val a66 = "line"
    val a67 = "line"
    val a68 = "line"
    val a69 = "line"
    val a70 = "line"
    val a71 = "line"
    val a72 = "line"
    val a73 = "line"
    val a74 = "line"
    val a75 = "line"
    val a76 = "line"
    val a77 = "line"
    val a78 = "line"
    val a79 = "line"
    val a80 = "line"
    val a81 = "line"
    val a82 = "line"
    val a83 = "line"
    val a84 = "line"
    val a85 = "line"
    val a86 = "line"
    val a87 = "line"
    val a88 = "line"
    val a89 = "line"
    val a90 = "line"
    val a91 = "line"
    val a92 = "line"
    val a93 = "line"
    val a94 = "line"
    val a95 = "line"
    val a96 = "line"
    val a97 = "line"
    println(a1 + a96 + a97)
}

// A function with 3 returns - under Circuit's max:4, should NOT be flagged
fun functionWithThreeReturns(x: Int): String {
    if (x < 0) {
        return "negative"
    }
    if (x == 0) {
        return "zero"
    }
    return "positive"
}
