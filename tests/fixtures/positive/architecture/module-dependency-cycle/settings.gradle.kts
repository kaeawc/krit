rootProject.name = "module-dependency-cycle-positive"

include(":a", ":b", ":c")
