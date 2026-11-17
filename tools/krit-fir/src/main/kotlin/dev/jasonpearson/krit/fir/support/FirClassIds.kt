package dev.jasonpearson.krit.fir.support

import org.jetbrains.kotlin.fir.types.ConeClassLikeType
import org.jetbrains.kotlin.fir.types.ConeStarProjection
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

/** Builds the [ClassId] of the nested class [relativeClassName] (`Outer.Inner`) in the dotted package [packageName]. */
fun nestedClassIdOf(packageName: String, relativeClassName: String): ClassId =
    ClassId(FqName(packageName), FqName(relativeClassName), isLocal = false)

/** The non-null type of this class with no type arguments. */
fun ClassId.classType(): ConeClassLikeType = constructClassLikeType(emptyArray(), isMarkedNullable = false)

/** The nullable type of this class with no type arguments, for `isSubtypeOf` checks that accept null. */
fun ClassId.nullableClassType(): ConeClassLikeType = constructClassLikeType(emptyArray(), isMarkedNullable = true)

/**
 * The nullable type of this class with [typeArgumentCount] star projections (`Map<*, *>?`), for `isSubtypeOf` checks
 * of a generic class that accept null.
 */
fun ClassId.nullableStarProjectedType(typeArgumentCount: Int): ConeClassLikeType =
    constructClassLikeType(Array(typeArgumentCount) { ConeStarProjection }, isMarkedNullable = true)
