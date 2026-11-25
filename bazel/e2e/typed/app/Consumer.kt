package typed.app

import typed.lib.Upstream

fun nameLength(upstream: Upstream): Int? = upstream.name?.length
