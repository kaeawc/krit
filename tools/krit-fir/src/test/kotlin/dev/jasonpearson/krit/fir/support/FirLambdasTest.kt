package dev.jasonpearson.krit.fir.support

import kotlin.test.Test
import kotlin.test.assertNull
import kotlin.test.assertSame
import org.jetbrains.kotlin.fir.FirModuleData
import org.jetbrains.kotlin.fir.FirSession
import org.jetbrains.kotlin.fir.declarations.FirDeclarationOrigin
import org.jetbrains.kotlin.fir.declarations.builder.buildAnonymousFunction
import org.jetbrains.kotlin.fir.expressions.FirAnonymousFunctionExpression
import org.jetbrains.kotlin.fir.expressions.FirExpression
import org.jetbrains.kotlin.fir.expressions.FirFunctionConversionKind
import org.jetbrains.kotlin.fir.expressions.builder.buildAnonymousFunctionExpression
import org.jetbrains.kotlin.fir.expressions.builder.buildFunctionTypeConversionExpression
import org.jetbrains.kotlin.fir.expressions.builder.buildLiteralExpression
import org.jetbrains.kotlin.fir.expressions.builder.buildNamedArgumentExpression
import org.jetbrains.kotlin.fir.symbols.impl.FirAnonymousFunctionSymbol
import org.jetbrains.kotlin.fir.types.impl.FirImplicitTypeRefImplWithoutSource
import org.jetbrains.kotlin.name.Name
import org.jetbrains.kotlin.platform.TargetPlatform
import org.jetbrains.kotlin.types.ConstantValueKind

class FirLambdasTest {
    // The builders only store the module data, so every member can stay unimplemented.
    private fun stubModuleData(): FirModuleData = object : FirModuleData() {
        override val name: Name get() = Name.identifier("stub")
        override val dependencies: List<FirModuleData> get() = emptyList()
        override val dependsOnDependencies: List<FirModuleData> get() = emptyList()
        override val allDependsOnDependencies: List<FirModuleData> get() = emptyList()
        override val friendDependencies: List<FirModuleData> get() = emptyList()
        override val platform: TargetPlatform get() = error("unused")
        override val isCommon: Boolean get() = false
        override val session: FirSession get() = error("unused")
        override val stableModuleName: String? get() = null
    }

    private fun lambda(trailing: Boolean): FirAnonymousFunctionExpression =
        buildAnonymousFunctionExpression {
            anonymousFunction = buildAnonymousFunction {
                moduleData = stubModuleData()
                origin = FirDeclarationOrigin.Synthetic.FakeFunction
                symbol = FirAnonymousFunctionSymbol()
                returnTypeRef = FirImplicitTypeRefImplWithoutSource
                isLambda = true
                hasExplicitParameterList = false
            }
            isTrailingLambda = trailing
        }

    private fun converted(expression: FirExpression): FirExpression =
        buildFunctionTypeConversionExpression {
            this.expression = expression
            kind = FirFunctionConversionKind.Sam
        }

    @Test
    fun trailingLambdaIsReturned() {
        val lambda = lambda(trailing = true)
        assertSame(lambda, trailingLambdaOf(lambda))
    }

    @Test
    fun lambdaInsideParenthesesIsNotTrailing() {
        assertNull(trailingLambdaOf(lambda(trailing = false)))
    }

    @Test
    fun lambdaArgumentIncludesNonTrailingAndSamConvertedLambdas() {
        val lambda = lambda(trailing = false)
        assertSame(lambda, lambdaArgumentOf(lambda))
        assertSame(lambda, lambdaArgumentOf(converted(lambda)))
    }

    @Test
    fun samConversionIsLookedThrough() {
        val lambda = lambda(trailing = true)
        assertSame(lambda, trailingLambdaOf(converted(lambda)))
    }

    @Test
    fun convertedNonTrailingLambdaIsNotTrailing() {
        assertNull(trailingLambdaOf(converted(lambda(trailing = false))))
    }

    @Test
    fun namedWrapperIsNotLookedThrough() {
        val named = buildNamedArgumentExpression {
            expression = lambda(trailing = true)
            isSpread = false
            name = Name.identifier("action")
        }
        assertNull(trailingLambdaOf(named))
    }

    @Test
    fun otherExpressionsAreNotLambdas() {
        val literal = buildLiteralExpression(source = null, kind = ConstantValueKind.Null, value = null, setType = false)
        assertNull(trailingLambdaOf(literal))
    }
}
