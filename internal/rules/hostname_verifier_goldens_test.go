package rules_test

import (
	"testing"
)

func TestBadHostnameVerifierFormerGoldens(t *testing.T) {
	for _, test := range []struct {
		name, source, want string
	}{
		{"BadHostnameVerifier", `





package test

import javax.net.ssl.HostnameVerifier
import javax.net.ssl.SSLSession

class AllowAllExpression : HostnameVerifier {
    override fun verify(hostname: String, session: SSLSession): Boolean = true
}

class AllowAllBlock : HostnameVerifier {
    override fun verify(hostname: String, session: SSLSession): Boolean {
        return true
    }
}

class AllowAllSemicolon : HostnameVerifier {
    override fun verify(hostname: String, session: SSLSession): Boolean {
        return true;
    }
}

class AllowAllCommented : HostnameVerifier {
    override fun verify(hostname: String, session: SSLSession): Boolean {
        // TODO: pin the production host
        /* accept everything for now */
        return true
    }
}

class AllowAllNullable : HostnameVerifier {
    override fun verify(hostname: String?, session: SSLSession?) = true
}

class AllowAllQualified : javax.net.ssl.HostnameVerifier {
    override fun verify(hostname: String, session: SSLSession): Boolean = true
}

class AllowAllAnnotated : HostnameVerifier {
    /**
     * KDoc is not part of the reported line.
     */
    @Suppress("UNUSED_PARAMETER")
    override fun verify(hostname: String, session: SSLSession): Boolean = true
}

class AllowAllMultiline : HostnameVerifier {
    override fun verify(
        hostname: String,
        session: SSLSession
    ): Boolean =
        true
}

open class AllowAllOpen : HostnameVerifier {
    final override fun verify(hostname: String, session: SSLSession): Boolean = true
}

abstract class AllowAllAbstract : HostnameVerifier, Cloneable {
    override fun verify(hostname: String, session: SSLSession): Boolean = true
}

interface AllowAllDefault : HostnameVerifier {
    override fun verify(hostname: String, session: SSLSession): Boolean = true
}

enum class AllowAllEnum : HostnameVerifier {
    INSTANCE;

    override fun verify(hostname: String, session: SSLSession): Boolean = true
}

enum class AllowAllEntries : HostnameVerifier {
    // FIR inherits the enum's verifier interface for the entry body; Go does
    // not infer an entry's implicit superclass and leaves this use silent.
    LENIENT {
        override fun verify(hostname: String, session: SSLSession): Boolean = true
    },
    STRICT {
        override fun verify(hostname: String, session: SSLSession): Boolean = hostname == session.peerHost
    },
}

class Outer {
    class NestedAllowAll : HostnameVerifier {
        override fun verify(hostname: String, session: SSLSession): Boolean = true
    }

    inner class InnerAllowAll : HostnameVerifier {
        override fun verify(hostname: String, session: SSLSession) = true
    }
}

fun localVerifier(): HostnameVerifier {
    class LocalAllowAll : HostnameVerifier {
        override fun verify(hostname: String, session: SSLSession): Boolean = true
    }
    return LocalAllowAll()
}


class Factory : HostnameVerifier {
    override fun verify(hostname: String, session: SSLSession): Boolean = hostname == session.peerHost

    fun lenient(): HostnameVerifier = object : HostnameVerifier {
        override fun verify(hostname: String, session: SSLSession): Boolean = true
    }
}`, "13:18,17:18,23:18,29:18,37:18,41:18,49:18,53:18,61:24,65:18,69:18,91:22,95:22,101:22,108:18,111:22"},
		{"BadHostnameVerifierCrossFile", `



package test

import fixture.hostname.VerifierBase
import javax.net.ssl.SSLSession

class Derived : VerifierBase() {
    override fun verify(host: String, session: SSLSession) = true
}`, ""},
		{"BadHostnameVerifierDivergence", `



package test

import javax.net.ssl.HostnameVerifier
import javax.net.ssl.SSLSession


class OverloadFirst : HostnameVerifier {
    fun verify(first: Int, second: Int): Boolean = true

    override fun verify(hostname: String, session: SSLSession): Boolean = true
}


class Holder(val delegate: HostnameVerifier) {
    fun verify(hostname: String, session: SSLSession): Boolean = true
}


class ComparesToTrue : HostnameVerifier {
    override fun verify(hostname: String, session: SSLSession): Boolean = hostname.isEmpty() == true
}


class LocalFunction : HostnameVerifier {
    override fun verify(hostname: String, session: SSLSession): Boolean = hostname == session.peerHost

    fun check(host: String): Boolean {
        fun verify(hostname: String, session: SSLSession): Boolean = true
        return host.isNotEmpty()
    }
}


class CompanionHelper : HostnameVerifier {
    override fun verify(hostname: String, session: SSLSession): Boolean = hostname == session.peerHost

    companion object {
        fun verify(hostname: String, session: SSLSession): Boolean = true
    }
}


class GenericOverloadFirst : HostnameVerifier {
    fun <T> verify(first: T, session: SSLSession): Boolean = true

    override fun verify(hostname: String, session: SSLSession): Boolean = hostname == session.peerHost
}`, "14:18,39:18"},
		{"BadHostnameVerifierGoMisses", `



package test

import javax.net.ssl.HostnameVerifier
import javax.net.ssl.SSLSession

object AllowAllObject : HostnameVerifier {
    override fun verify(hostname: String, session: SSLSession): Boolean = true
}

class WithCompanion {
    companion object : HostnameVerifier {
        override fun verify(hostname: String, session: SSLSession): Boolean = true
    }
}

val topLevelAnonymous: HostnameVerifier = object : HostnameVerifier {
    override fun verify(hostname: String, session: SSLSession): Boolean = true
}

fun anonymousInFunction(): HostnameVerifier = object : HostnameVerifier {
    override fun verify(hostname: String, session: SSLSession): Boolean = true
}

abstract class VerifierBase : HostnameVerifier

class DerivedAllowAll : VerifierBase() {
    override fun verify(hostname: String, session: SSLSession): Boolean = true
}

typealias Verifier = HostnameVerifier

class AliasedAllowAll : Verifier {
    override fun verify(hostname: String, session: SSLSession): Boolean = true
}


class TwoVerifiers : HostnameVerifier {
    override fun verify(hostname: String, session: SSLSession): Boolean = true

    companion object Lenient : HostnameVerifier {
        override fun verify(hostname: String, session: SSLSession): Boolean = true
    }
}

class LabeledReturn : HostnameVerifier {
    override fun verify(hostname: String, session: SSLSession): Boolean {
        return@verify true
    }
}

class Parenthesized : HostnameVerifier {
    override fun verify(hostname: String, session: SSLSession): Boolean = (true)
}

class Backticked : HostnameVerifier {
    override fun ` + "`verify`" + `(hostname: String, session: SSLSession): Boolean = true
}

class TrailingComma : HostnameVerifier {
    override fun verify(
        hostname: String,
        session: SSLSession,
    ): Boolean = true
}

class ParenthesizedReturn : HostnameVerifier {
    override fun verify(hostname: String, session: SSLSession): Boolean {
        return (true)
    }
}`, "21:18,25:18,37:18,42:18,50:18,64:18"},
		{"BadHostnameVerifierImportAlias", `


package test

import javax.net.ssl.HostnameVerifier as Checker
import javax.net.ssl.SSLSession

class AliasImportAllowAll : Checker {
    override fun verify(hostname: String, session: SSLSession): Boolean = true
}`, "10:18"},
		{"BadHostnameVerifierInheritance", `



package test

import javax.net.ssl.HostnameVerifier
import javax.net.ssl.SSLSession

class Scope {
    abstract class Base : HostnameVerifier
    class Nested : Base() {
        override fun verify(host: String, session: SSLSession) = true
    }
}

class Qualified : Scope.Base() {
    override fun verify(host: String, session: SSLSession) = true
}

abstract class Root : HostnameVerifier
abstract class First : Root()
abstract class Second : First()
abstract class Third : Second()
abstract class Fourth : Third()
abstract class Fifth : Fourth()
abstract class Sixth : Fifth()
class Deep : Sixth() {
    override fun verify(host: String, session: SSLSession) = true
}`, ""},
		{"BadHostnameVerifierLambda", `



package test

import javax.net.ssl.HostnameVerifier

val trustAll = HostnameVerifier { _, _ -> true }
val parenthesized = HostnameVerifier { _, _ -> (true) }
val explicitReturn = HostnameVerifier { _, _ -> return@HostnameVerifier true }
val argument = HostnameVerifier({ _, _ -> true })`, "9:16,10:21,11:22,12:16"},
		{"BadHostnameVerifierLookalike", `



package test

import javax.net.ssl.SSLSession

interface HostnameVerifier {
    fun verify(hostname: String, session: SSLSession): Boolean
}

class LocalVerifier : HostnameVerifier {
    override fun verify(hostname: String, session: SSLSession): Boolean = true
}


class JdkVerifier : javax.net.ssl.HostnameVerifier {
    override fun verify(hostname: String, session: SSLSession): Boolean = true
}`, "19:18"},
		{"BadHostnameVerifierNegative", `



package test

import javax.net.ssl.HostnameVerifier
import javax.net.ssl.SSLSession

class Validating : HostnameVerifier {
    override fun verify(hostname: String, session: SSLSession): Boolean = hostname == session.peerHost
}

class Delegating(delegate: HostnameVerifier) : HostnameVerifier by delegate

class RejectAll : HostnameVerifier {
    override fun verify(hostname: String, session: SSLSession): Boolean = false
}

class ExtensionHelper : HostnameVerifier {
    override fun verify(hostname: String, session: SSLSession) = hostname == session.peerHost
    fun Any.verify(hostname: String, session: SSLSession) = true
    fun <Value> verify(hostname: Value, session: SSLSession) = true
    fun verify(hostname: String, vararg session: SSLSession) = true
}



class LogsFirst : HostnameVerifier {
    override fun verify(hostname: String, session: SSLSession): Boolean {
        println(hostname)
        return true
    }
}



class DeclaresFirst : HostnameVerifier {
    override fun verify(hostname: String, session: SSLSession): Boolean {
        val values = listOf(1)
        return true
    }
}

class ThrowsFirst : HostnameVerifier {
    override fun verify(hostname: String, session: SSLSession): Boolean {
        if (hostname.isEmpty()) throw IllegalArgumentException()
        return true
    }
}

class Conditional : HostnameVerifier {
    override fun verify(hostname: String, session: SSLSession): Boolean {
        if (hostname.isEmpty()) return false
        return true
    }
}


class NonlocalReturn : HostnameVerifier {
    override fun verify(hostname: String, session: SSLSession): Boolean {
        run { if (hostname.isEmpty()) return false }
        return true
    }
}

class Branches : HostnameVerifier {
    override fun verify(hostname: String, session: SSLSession): Boolean =
        if (hostname.isEmpty()) true else hostname == session.peerHost
}

private const val ALWAYS = true

class ConstantResult : HostnameVerifier {
    override fun verify(hostname: String, session: SSLSession): Boolean = ALWAYS
}

class Wrapped : HostnameVerifier {
    override fun verify(hostname: String, session: SSLSession): Boolean = run { true }
}

abstract class AbstractVerifier : HostnameVerifier {
    abstract override fun verify(hostname: String, session: SSLSession): Boolean
}

interface NoDefault : HostnameVerifier


class NotAVerifier {
    fun verify(hostname: String, session: SSLSession): Boolean = true
}


fun verify(hostname: String, session: SSLSession): Boolean = true


val validatingLambda = HostnameVerifier { host, _ -> host == "example.com" }



class AnonymousHelper : HostnameVerifier {
    override fun verify(hostname: String, session: SSLSession): Boolean = hostname == session.peerHost

    val probe = object {
        fun verify(first: String, second: String): Boolean = true
    }
}`, "21:18,30:18,39:18,46:18,53:18,61:18,102:18"},
		{"BadHostnameVerifierOwnerBoundary", `



package test

import javax.net.ssl.HostnameVerifier
import javax.net.ssl.SSLSession

class Strict() : HostnameVerifier {
    override fun verify(host: String, session: SSLSession) = host == session.peerHost

    init {
        fun verify(host: String, session: SSLSession) = true
    }

    constructor(value: Int) : this() {
        fun verify(host: String, session: SSLSession) = true
    }

    var probe: Boolean = false
        get() {
            fun verify(host: String, session: SSLSession) = true
            return false
        }
        set(value) {
            fun verify(host: String, session: SSLSession) = true
        }

    class Helper { fun verify(host: String, session: SSLSession) = true }
    interface Fallback { fun verify(host: String, session: SSLSession) = true }
    object Local { fun verify(host: String, session: SSLSession) = true }
}`, ""},
		{"BadHostnameVerifierSamConversion", `



package test.allowallhostnameverifiersamconversion

import javax.net.ssl.HttpsURLConnection
import okhttp3.OkHttpClient

fun installDefault() {
    HttpsURLConnection.setDefaultHostnameVerifier { _, _ -> true }
}

fun okHttp(): OkHttpClient.Builder = OkHttpClient.Builder().hostnameVerifier { _, _ -> true }

fun parenthesized(connection: HttpsURLConnection) {
    connection.setHostnameVerifier({ hostname, session -> true })
}

fun named(): OkHttpClient.Builder =
    OkHttpClient.Builder().hostnameVerifier(hostnameVerifier = { _, _ -> true })


fun checksHost() {
    HttpsURLConnection.setDefaultHostnameVerifier { hostname, _ -> hostname == "example.com" }
}


fun otherInterfaces(list: ArrayList<String>): java.util.function.BiPredicate<String, String> {
    list.removeIf { true }
    return java.util.function.BiPredicate { _, _ -> true }
}`, ""},
		{"BadHostnameVerifierSamLookalike", `




package test

import javax.net.ssl.HostnameVerifier
import javax.net.ssl.SSLSession

fun HostnameVerifier(predicate: (String, SSLSession) -> Boolean): HostnameVerifier =
    javax.net.ssl.HostnameVerifier { host, _ -> host == "example.com" }

val factoryResult = HostnameVerifier(predicate = { _, _ -> true })
val actualSam = HostnameVerifier { _, _ -> true }`, ""},
		{"BadHostnameVerifierShadow", `



package test

import javax.net.ssl.HostnameVerifier
import javax.net.ssl.SSLSession

fun helper(HostnameVerifier: Any) = HostnameVerifier.toString()

class Real : HostnameVerifier {
    override fun verify(host: String, session: SSLSession) = true
}`, "13:18"},
		{"BadHostnameVerifierStarImport", `


package test

import javax.net.ssl.*

class StarImportAllowAll : HostnameVerifier {
    override fun verify(hostname: String, session: SSLSession): Boolean = true
}`, "9:18"},
		{"BadHostnameVerifierTypeAlias", `



package test

import javax.net.ssl.HostnameVerifier
import javax.net.ssl.SSLSession

typealias Session = SSLSession
typealias Verifier = HostnameVerifier
typealias ChainedVerifier = Verifier

class AliasedParameter : HostnameVerifier {
    override fun verify(host: String, session: Session) = true
}

class Chained : ChainedVerifier {
    override fun verify(host: String, session: SSLSession) = true
}

val aliasedSam = Verifier { _, _ -> true }`, ""},
	} {
		t.Run(test.name, func(t *testing.T) {
			findings := runRuleByName(t, "BadHostnameVerifier", test.source)
			if got := findingPositions(findings); got != test.want {
				t.Fatalf("positions = %s, want %s", got, test.want)
			}
			if findings := runRuleByName(t, "AllowAllHostnameVerifier", test.source); len(findings) != 0 {
				t.Fatalf("custom verifiers must not receive Apache findings: %v", findings)
			}
		})
	}
}
