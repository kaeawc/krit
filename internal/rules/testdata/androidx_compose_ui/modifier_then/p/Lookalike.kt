package p.thenlookalike

class Chain {
    fun then(other: Chain): Chain = other
    fun next(): Chain = this
    fun build(): Chain = then(next())
}
