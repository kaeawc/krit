package dev.jasonpearson.krit.fir.support

import org.jetbrains.kotlin.fir.types.ConeClassLikeType
import org.jetbrains.kotlin.fir.types.constructClassLikeType
import org.jetbrains.kotlin.name.ClassId
import org.jetbrains.kotlin.name.FqName
import org.jetbrains.kotlin.name.Name

/** Builds the top-level [ClassId] for [className] in the dotted package [packageName]. */
fun classIdOf(packageName: String, className: String): ClassId =
    ClassId(FqName(packageName), Name.identifier(className))

/** Builds the top-level [ClassId] for [className] in [packageName]. */
fun classIdOf(packageName: FqName, className: String): ClassId =
    ClassId(packageName, Name.identifier(className))

/** The nullable type of this class with no type arguments, for `isSubtypeOf` checks that accept null. */
fun ClassId.nullableClassType(): ConeClassLikeType = constructClassLikeType(emptyArray(), isMarkedNullable = true)
