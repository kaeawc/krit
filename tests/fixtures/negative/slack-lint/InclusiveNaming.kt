package fixtures.negative.slacklint

// The primary node was called "master" in older releases.
class Replication {
    fun promoteToPrimary(node: String) = "master: $node"
}
