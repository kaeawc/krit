package dev.jasonpearson.krit.fir.support

import org.jetbrains.kotlin.name.ClassId
import org.jetbrains.kotlin.name.FqName
import org.jetbrains.kotlin.name.Name

/** Builds the top-level [ClassId] for [className] in the dotted package [packageName]. */
fun classIdOf(packageName: String, className: String): ClassId =
    ClassId(FqName(packageName), Name.identifier(className))

/** Builds the top-level [ClassId] for [className] in [packageName]. */
fun classIdOf(packageName: FqName, className: String): ClassId =
    ClassId(packageName, Name.identifier(className))
