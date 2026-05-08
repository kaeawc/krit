package com.example.releaseengineering

val x = 1
val y = 2

// Indented marker-like strings inside comments and code should not flag.
val a = "<<<<<<< HEAD"
val b = "======="
val c = ">>>>>>> feature"

// Markers inside a line comment (not at column 0) — not a real conflict.
    // <<<<<<< HEAD

/*
 * Documentation showing how merge conflicts look:
 * <<<<<<< HEAD
 * =======
 * >>>>>>> feature
 */

val rawDoc = """
<<<<<<< HEAD
some content
=======
other content
>>>>>>> feature
"""

val moreEquals = "==========="
