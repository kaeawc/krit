rootProject.name = "layer-dependency-violation-positive"

include(":ui", ":domain", ":data-api", ":data-internal")

project(":data-api").projectDir = file("data/api")
project(":data-internal").projectDir = file("data/internal")
