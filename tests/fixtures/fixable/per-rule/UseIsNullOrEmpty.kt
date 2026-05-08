package fixtures.positive.style

fun checkName(name: String?) {
    if (name == null || name.isEmpty()) {
        println("missing name")
    }
}

fun checkScores(scores: List<Int>?) {
    if (scores == null || scores.count() == 0) {
        println("missing scores")
    }
}

fun checkIds(ids: List<Int>?) {
    if (ids == null || ids.size == 0) {
        println("missing ids")
    }
}

fun checkLabel(label: String?) {
    if (label == null || label.length == 0) {
        println("missing label")
    }
}

fun checkToken(token: String?) {
    if (token == null || token == "") {
        println("missing token")
    }
}
