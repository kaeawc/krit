package dev.jasonpearson.krit.fir.support

import kotlin.test.Test
import kotlin.test.assertNull
import kotlin.test.assertSame
import org.jetbrains.kotlin.fir.FirContractViolation
import org.jetbrains.kotlin.fir.FirExpressionRef
import org.jetbrains.kotlin.fir.expressions.FirExpression
import org.jetbrains.kotlin.fir.expressions.builder.buildArgumentList
import org.jetbrains.kotlin.fir.expressions.builder.buildCheckNotNullCall
import org.jetbrains.kotlin.fir.expressions.builder.buildCheckedSafeCallSubject
import org.jetbrains.kotlin.fir.expressions.builder.buildLiteralExpression
import org.jetbrains.kotlin.fir.expressions.builder.buildPropertyAccessExpression
import org.jetbrains.kotlin.fir.expressions.builder.buildSmartCastExpression
import org.jetbrains.kotlin.fir.references.builder.buildSimpleNamedReference
import org.jetbrains.kotlin.fir.symbols.impl.ConeClassLikeLookupTagImpl
import org.jetbrains.kotlin.fir.types.ConeClassLikeType
import org.jetbrains.kotlin.fir.types.ConeTypeProjection
import org.jetbrains.kotlin.fir.types.builder.buildResolvedTypeRef
import org.jetbrains.kotlin.fir.types.impl.ConeClassLikeTypeImpl
import org.jetbrains.kotlin.name.ClassId
import org.jetbrains.kotlin.name.FqName
import org.jetbrains.kotlin.name.Name
import org.jetbrains.kotlin.types.ConstantValueKind
import org.jetbrains.kotlin.types.SmartcastStability

class FirSmartCastsTest {
    private val access: FirExpression = buildPropertyAccessExpression {
        calleeReference = buildSimpleNamedReference { name = Name.identifier("x") }
    }

    private val literal: FirExpression =
        buildLiteralExpression(source = null, kind = ConstantValueKind.Null, value = null, setType = false)

    private fun smartCast(original: FirExpression): FirExpression {
        val type = ConeClassLikeTypeImpl(
            ConeClassLikeLookupTagImpl(ClassId(FqName("kotlin"), Name.identifier("Any"))),
            emptyArray<ConeTypeProjection>(),
            false,
        )
        val ref = buildResolvedTypeRef { coneType = type }
        return buildSmartCastExpression {
            originalExpression = original
            smartcastType = ref
            smartcastTypeWithoutNullableNothing = ref
            smartcastStability = SmartcastStability.STABLE_VALUE
            upperTypesFromSmartCast = emptyList()
            lowerTypesFromSmartCast = emptyList()
        }
    }

    @OptIn(FirContractViolation::class)
    private fun safeCallSubject(original: FirExpression): FirExpression = buildCheckedSafeCallSubject {
        originalReceiverRef = FirExpressionRef<FirExpression>().apply { bind(original) }
    }

    private fun notNull(argument: FirExpression): FirExpression = buildCheckNotNullCall {
        argumentList = buildArgumentList { arguments += argument }
    }

    @Test
    fun plainExpressionIsReturnedUnchanged() {
        assertSame(access, unwrapSmartCasts(access))
        assertSame(literal, unwrapSmartCasts(literal, throughNotNull = true))
    }

    @Test
    fun smartCastAndSafeCallSubjectAreStrippedInAnyOrder() {
        assertSame(access, unwrapSmartCasts(smartCast(access)))
        assertSame(access, unwrapSmartCasts(safeCallSubject(access)))
        assertSame(access, unwrapSmartCasts(smartCast(safeCallSubject(smartCast(access)))))
        assertSame(access, unwrapSmartCasts(safeCallSubject(smartCast(safeCallSubject(access)))))
    }

    @Test
    fun notNullAssertionIsStrippedOnlyOnRequest() {
        val asserted = notNull(access)
        assertSame(asserted, unwrapSmartCasts(asserted))
        assertSame(access, unwrapSmartCasts(asserted, throughNotNull = true))
        assertSame(access, unwrapSmartCasts(smartCast(notNull(safeCallSubject(access))), throughNotNull = true))
    }

    @Test
    fun notNullAssertionWithoutArgumentIsKept() {
        val empty = buildCheckNotNullCall { argumentList = buildArgumentList {} }
        assertSame(empty, unwrapSmartCasts(empty, throughNotNull = true))
    }

    @Test
    fun absentExpressionStaysAbsent() {
        assertNull(unwrapSmartCasts(null as FirExpression?))
        assertSame(access, unwrapSmartCasts(smartCast(access) as FirExpression?))
    }
}
