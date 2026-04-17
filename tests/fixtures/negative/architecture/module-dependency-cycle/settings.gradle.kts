rootProject.name = "module-dependency-cycle-negative"

include(":a", ":b", ":c")
