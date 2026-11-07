package dev.jasonpearson.krit.fir.checkers.performance

import dev.jasonpearson.krit.fir.FirRule
import dev.jasonpearson.krit.fir.report
import dev.jasonpearson.krit.fir.checkers.initializerInCheckedFile
import org.jetbrains.kotlin.diagnostics.DiagnosticReporter
import org.jetbrains.kotlin.fir.analysis.checkers.MppCheckerKind
import org.jetbrains.kotlin.fir.analysis.checkers.context.CheckerContext
import org.jetbrains.kotlin.fir.analysis.checkers.expression.ExpressionCheckers
import org.jetbrains.kotlin.fir.analysis.checkers.expression.FirFunctionCallChecker
import org.jetbrains.kotlin.fir.expressions.FirExpression
import org.jetbrains.kotlin.fir.expressions.FirFunctionCall
import org.jetbrains.kotlin.fir.expressions.FirPropertyAccessExpression
import org.jetbrains.kotlin.fir.symbols.impl.FirClassLikeSymbol
import org.jetbrains.kotlin.fir.symbols.impl.FirPropertySymbol
import org.jetbrains.kotlin.fir.references.toResolvedCallableSymbol
import org.jetbrains.kotlin.fir.symbols.impl.FirFunctionSymbol
import org.jetbrains.kotlin.fir.symbols.impl.FirNamedFunctionSymbol
import org.jetbrains.kotlin.fir.types.ConeClassLikeType
import org.jetbrains.kotlin.fir.types.ConeKotlinType
import org.jetbrains.kotlin.fir.types.lowerBoundIfFlexible
import org.jetbrains.kotlin.name.ClassId
import org.jetbrains.kotlin.name.FqName
import org.jetbrains.kotlin.name.Name

// Flags a call to one of android.graphics.BitmapFactory's decode methods that
// takes no BitmapFactory.Options, so the bitmap is decoded at full size:
// `BitmapFactory.decodeFile(path)` and `BitmapFactory.decodeStream(stream)`.
// Passing Options (even `null`) selects the other overload and is not
// reported, as Go does not report it.
//
// Like the Go rule, the finding sits on the first line of the call
// expression (the receiver's line for a qualified call) and names the called
// method.
//
// Decodes inside a file, class or function named for Thumb, Tiny, Icon or Avatar, and decodeByteArray of
// a header-plus-tail concatenation, are small images and are not reported.
//
// Deliberate differences from Go, each pinned in the golden data:
// - Recall (Go misses these true positives): Go counts one value argument, so
//   the Options-less `decodeResource(res, id)`, `decodeByteArray(data, off,
//   len)`, and `decodeFileDescriptor(fd)` overloads are reported here too
//   (Go's method list stops at decodeFile, decodeResource, and decodeStream,
//   and decodeResource without Options takes two arguments). Go also needs
//   the receiver spelled `BitmapFactory` or `<qualifier>.BitmapFactory`, so
//   an import alias, a typealias, and a statically imported decode method
//   are reported here and missed by Go.
// - Precision: Go matches the receiver by its last segment of text, so it
//   also reports a declaration of the user's own that is only named
//   BitmapFactory. This checker requires the call to resolve to
//   android.graphics.BitmapFactory.
internal object BitmapDecodeWithoutOptions : FirFunctionCallChecker(MppCheckerKind.Common), FirRule {
    override val ruleId = "BitmapDecodeWithoutOptions"
    override val expressionCheckers = object : ExpressionCheckers() {
        override val functionCallCheckers = setOf(BitmapDecodeWithoutOptions)
    }

    private val bitmapFactory = ClassId(FqName("android.graphics"), Name.identifier("BitmapFactory"))
    private val options = ClassId(FqName("android.graphics"), FqName("BitmapFactory.Options"), false)
    private val smallImageNames = listOf("Thumb", "Tiny", "Icon", "Avatar")
    private val decodeMethods = setOf(
        "decodeFile",
        "decodeResource",
        "decodeStream",
        "decodeByteArray",
        "decodeFileDescriptor",
    )

    context(context: CheckerContext, reporter: DiagnosticReporter)
    override fun check(expression: FirFunctionCall) {
        val callee = expression.calleeReference.toResolvedCallableSymbol() as? FirFunctionSymbol<*> ?: return
        val callableId = callee.callableId
        if (callableId.classId != bitmapFactory) return
        val method = callableId.callableName.asString()
        if (method !in decodeMethods) return
        if (callee.valueParameterSymbols.any { isOptions(it.resolvedReturnType) }) return
        if (inSmallImageContext() || (method == "decodeByteArray" && isConcatenatedBytes(expression.argumentList.arguments.firstOrNull()))) return
        report(
            expression.source,
            "BitmapFactory.$method without BitmapFactory.Options may decode a full-size bitmap. " +
                "Pass BitmapFactory.Options to control memory usage.",
        )
    }

    // A thumbnail, tiny or avatar decoder (by file, class or function name) handles small images.
    context(context: CheckerContext)
    private fun inSmallImageContext(): Boolean {
        val names = buildList {
            context.containingFile?.name?.let(::add)
            for (symbol in context.containingDeclarations) {
                when (symbol) {
                    is FirNamedFunctionSymbol -> add(symbol.name.asString())
                    is FirClassLikeSymbol<*> -> add(symbol.name.asString())
                    else -> Unit
                }
            }
        }
        return names.any { name -> smallImageNames.any { name.contains(it, ignoreCase = true) } }
    }

    // A fixed header joined to a short tail, such as an image prefix plus a payload, is a small decode.
    context(context: CheckerContext)
    private fun isConcatenatedBytes(argument: FirExpression?): Boolean {
        val value = when (argument) {
            is FirPropertyAccessExpression -> (argument.calleeReference.toResolvedCallableSymbol() as? FirPropertySymbol)?.initializerInCheckedFile()
            else -> argument
        }
        val call = value as? FirFunctionCall ?: return false
        return call.calleeReference.toResolvedCallableSymbol()?.callableId?.callableName?.asString() == "plus"
    }

    private fun isOptions(type: ConeKotlinType): Boolean =
        (type.lowerBoundIfFlexible() as? ConeClassLikeType)?.lookupTag?.classId == options
}
