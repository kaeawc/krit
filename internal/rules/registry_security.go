package rules

import (
	"strings"

	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/rules/api/evidence"
	"github.com/kaeawc/krit/internal/scanner"
)

func registerSecurityRules() {

	// --- from security.go ---
	{
		r := &ContentProviderQueryWithSelectionInterpolationRule{BaseRule: BaseRule{RuleName: "ContentProviderQueryWithSelectionInterpolation", RuleSetName: "security", Sev: "info", Desc: "Detects interpolated selection strings passed to ContentResolver.query() that may enable SQL injection."}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			NodeTypes:          []string{"call_expression"},
			LexicalCalleeNames: []string{"query"},
			Needs:              api.NeedsResolver,
			Confidence:         0.75, Implementation: r,
			Check: func(ctx *api.Context) {
				idx, file := ctx.Idx, ctx.File
				ev := evidence.From(ctx)
				call := ev.Call(idx)
				if call == nil || call.Callee != "query" {
					return
				}
				fqn, source := ev.ResolveOwner(call)
				if source == evidence.OwnerUnknown || fqn != "android.content.ContentResolver" {
					return
				}
				_, args := flatCallExpressionParts(file, idx)
				if args == 0 {
					return
				}
				selectionArg := flatNamedValueArgument(file, args, "selection")
				if selectionArg == 0 {
					selectionArg = flatPositionalValueArgument(file, args, 2)
				}
				if selectionArg == 0 || !flatContainsStringInterpolation(file, selectionArg) {
					return
				}
				ctx.EmitAt(file.FlatRow(selectionArg)+1, file.FlatCol(selectionArg)+1, "Interpolated ContentResolver selection string. Use selectionArgs placeholders instead.")
			},
		})
	}
	{
		r := &SQLInjectionRawQueryRule{BaseRule: BaseRule{RuleName: "SqlInjectionRawQuery", RuleSetName: "security", Sev: "info", Desc: "Detects SQLiteDatabase SQL arguments built with interpolation or non-static concatenation."}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			NodeTypes:          []string{"call_expression", "method_invocation"},
			Languages:          []scanner.Language{scanner.LangKotlin, scanner.LangJava},
			LexicalCalleeNames: []string{"rawQuery", "execSQL", "query"},
			Needs:              api.NeedsResolver,
			Confidence:         r.Confidence(), Implementation: r,
			Check: func(ctx *api.Context) {
				idx, file := ctx.Idx, ctx.File
				name := sqlInjectionCallName(file, idx)
				if name != "rawQuery" && name != "execSQL" && name != "query" {
					return
				}
				ev := evidence.From(ctx)
				call := ev.Call(idx)
				if call == nil {
					return
				}
				fqn, source := ev.ResolveOwner(call)
				if source == evidence.OwnerUnknown || !isSQLiteDatabaseFQN(fqn) {
					return
				}
				sqlArg := sqlInjectionSQLArgument(file, idx, name)
				switch argumentIsUntrustedShape(file, sqlArg) {
				case sqlArgumentInterpolated:
					ctx.EmitAt(file.FlatRow(sqlArg)+1, file.FlatCol(sqlArg)+1, "SQLite SQL argument uses string interpolation. Use placeholders and bind arguments instead.")
				case sqlArgumentComputed:
					ctx.EmitAt(file.FlatRow(sqlArg)+1, file.FlatCol(sqlArg)+1, "SQLite SQL argument is built with non-static concatenation. Use placeholders and bind arguments instead.")
				}
			},
		})
	}
	{
		r := &RuntimeExecUnsafeShapeRule{BaseRule: BaseRule{RuleName: "RuntimeExecUnsafeShape", RuleSetName: "security", Sev: "info", Desc: "Detects Runtime.getRuntime().exec(String) commands built with interpolation or non-static concatenation."}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			NodeTypes:          []string{"call_expression", "method_invocation"},
			Languages:          []scanner.Language{scanner.LangKotlin, scanner.LangJava},
			LexicalCalleeNames: []string{"exec"},
			Needs:              api.NeedsResolver,
			Confidence:         r.Confidence(), Implementation: r,
			Check: func(ctx *api.Context) {
				idx, file := ctx.Idx, ctx.File
				ev := evidence.From(ctx)
				call := ev.Call(idx)
				if call == nil || call.Callee != "exec" || call.ReceiverIdx == 0 {
					return
				}
				inner := ev.Call(call.ReceiverIdx)
				if inner == nil || inner.Callee != "getRuntime" {
					return
				}
				fqn, source := ev.ResolveOwner(inner)
				if source == evidence.OwnerUnknown || fqn != "java.lang.Runtime" {
					return
				}
				commandArg := runtimeExecSingleArgument(file, idx)
				switch argumentIsUntrustedShape(file, commandArg) {
				case sqlArgumentInterpolated:
					ctx.EmitAt(file.FlatRow(commandArg)+1, file.FlatCol(commandArg)+1, "Runtime.exec(String) uses an interpolated command string. Pass a String array or ProcessBuilder argument list instead.")
				case sqlArgumentComputed:
					ctx.EmitAt(file.FlatRow(commandArg)+1, file.FlatCol(commandArg)+1, "Runtime.exec(String) uses a computed command string. Pass a String array or ProcessBuilder argument list instead.")
				}
			},
		})
	}
	{
		r := &RoomRawQueryStringConcatRule{BaseRule: BaseRule{RuleName: "RoomRawQueryStringConcat", RuleSetName: "security", Sev: "info", Desc: "Detects SimpleSQLiteQuery SQL strings built with interpolation or non-static concatenation without bind args."}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			NodeTypes:          []string{"call_expression"},
			Languages:          []scanner.Language{scanner.LangKotlin},
			LexicalCalleeNames: []string{"SimpleSQLiteQuery"},
			Needs:              api.NeedsResolver,
			Confidence:         r.Confidence(), Implementation: r,
			Check: func(ctx *api.Context) {
				idx, file := ctx.Idx, ctx.File
				ev := evidence.From(ctx)
				call := ev.Call(idx)
				if call == nil || call.Callee != "SimpleSQLiteQuery" || call.Receiver != "" {
					return
				}
				fqn, source := ev.ResolveCalleeFQN(call)
				if source == evidence.OwnerUnknown || fqn != "androidx.sqlite.db.SimpleSQLiteQuery" {
					return
				}
				sqlArg := roomRawQuerySQLArgWithoutBindArgs(file, idx)
				switch argumentIsUntrustedShape(file, sqlArg) {
				case sqlArgumentInterpolated:
					ctx.EmitAt(file.FlatRow(sqlArg)+1, file.FlatCol(sqlArg)+1, "SimpleSQLiteQuery SQL uses string interpolation without bind args. Use ? placeholders and pass bindArgs.")
				case sqlArgumentComputed:
					ctx.EmitAt(file.FlatRow(sqlArg)+1, file.FlatCol(sqlArg)+1, "SimpleSQLiteQuery SQL is built with non-static concatenation without bind args. Use ? placeholders and pass bindArgs.")
				}
			},
		})
	}
	{
		r := &ProcessBuilderShellArgRule{BaseRule: BaseRule{RuleName: "ProcessBuilderShellArg", RuleSetName: "security", Sev: "info", Desc: "Detects ProcessBuilder shell commands whose -c script argument is interpolated or computed."}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			NodeTypes:          []string{"call_expression", "object_creation_expression"},
			Languages:          []scanner.Language{scanner.LangKotlin, scanner.LangJava},
			LexicalCalleeNames: []string{"ProcessBuilder"},
			Needs:              api.NeedsResolver,
			Confidence:         r.Confidence(), Implementation: r,
			Check: func(ctx *api.Context) {
				idx, file := ctx.Idx, ctx.File
				ev := evidence.From(ctx)
				call := ev.Call(idx)
				if call == nil || call.Callee != "ProcessBuilder" || call.Receiver != "" {
					return
				}
				fqn, source := ev.ResolveCalleeFQN(call)
				if source == evidence.OwnerUnknown || fqn != "java.lang.ProcessBuilder" {
					return
				}
				scriptArg := processBuilderShellScriptArgument(file, idx)
				switch argumentIsUntrustedShape(file, scriptArg) {
				case sqlArgumentInterpolated:
					ctx.EmitAt(file.FlatRow(scriptArg)+1, file.FlatCol(scriptArg)+1, "ProcessBuilder shell -c argument uses string interpolation. Pass command and arguments separately without a shell.")
				case sqlArgumentComputed:
					ctx.EmitAt(file.FlatRow(scriptArg)+1, file.FlatCol(scriptArg)+1, "ProcessBuilder shell -c argument is built with non-static concatenation. Pass command and arguments separately without a shell.")
				}
			},
		})
	}
	{
		r := &LogPiiRule{
			BaseRule:       BaseRule{RuleName: "LogPii", RuleSetName: "security", Sev: "info", Desc: "Detects logger calls that include sensitive variable names in interpolated or concatenated messages."},
			PiiNamePattern: defaultLogPiiNamePattern,
		}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			NodeTypes:  []string{"call_expression", "method_invocation"},
			Languages:  []scanner.Language{scanner.LangKotlin, scanner.LangJava},
			Confidence: r.Confidence(), Implementation: r,
			Check: func(ctx *api.Context) {
				idx, file := ctx.Idx, ctx.File
				if !logPiiIsLoggerCall(file, idx) {
					return
				}
				arg := logPiiSensitiveArgument(file, idx, logPiiEffectivePattern(r))
				if arg == 0 {
					return
				}
				ctx.EmitAt(file.FlatRow(arg)+1, file.FlatCol(arg)+1, "Logger call includes a sensitive variable name in the message. Redact the value or omit it from logs.")
			},
		})
	}
	{
		r := &JdbcStatementExecuteRule{BaseRule: BaseRule{RuleName: "JdbcStatementExecute", RuleSetName: "security", Sev: "info", Desc: "Detects java.sql.Statement execution calls with interpolated or computed SQL strings."}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			NodeTypes:          []string{"call_expression", "method_invocation"},
			Languages:          []scanner.Language{scanner.LangKotlin, scanner.LangJava},
			LexicalCalleeNames: []string{"execute", "executeQuery", "executeUpdate", "executeLargeUpdate"},
			Needs:              api.NeedsResolver,
			Confidence:         r.Confidence(), Implementation: r,
			Check: func(ctx *api.Context) {
				idx, file := ctx.Idx, ctx.File
				name := jdbcStatementMethodName(file, idx)
				if !jdbcStatementExecuteMethods[name] {
					return
				}
				ev := evidence.From(ctx)
				call := ev.Call(idx)
				if call == nil {
					return
				}
				if !jdbcReceiverIsStatement(file, ev, call) {
					return
				}
				sqlArg := jdbcStatementSQLArgumentExpr(file, idx)
				switch argumentIsUntrustedShape(file, sqlArg) {
				case sqlArgumentInterpolated:
					ctx.EmitAt(file.FlatRow(sqlArg)+1, file.FlatCol(sqlArg)+1, "JDBC Statement SQL uses string interpolation. Use PreparedStatement with bind parameters.")
				case sqlArgumentComputed:
					ctx.EmitAt(file.FlatRow(sqlArg)+1, file.FlatCol(sqlArg)+1, "JDBC Statement SQL is built with non-static concatenation. Use PreparedStatement with bind parameters.")
				}
			},
		})
	}
	{
		r := &XMLExternalEntityRule{BaseRule: BaseRule{RuleName: "XmlExternalEntity", RuleSetName: "security", Sev: "warning", Desc: "Detects XML parser factories created without XXE-disabling hardening."}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			NodeTypes:  []string{"call_expression", "method_invocation"},
			Languages:  []scanner.Language{scanner.LangKotlin, scanner.LangJava},
			Confidence: r.Confidence(), Implementation: r,
			Check: func(ctx *api.Context) {
				idx, file := ctx.Idx, ctx.File
				if !xmlExternalEntityFactoryCall(file, idx) || xmlExternalEntityHasHardeningAfter(file, idx) {
					return
				}
				ctx.EmitAt(file.FlatRow(idx)+1, file.FlatCol(idx)+1, "XML factory created without XXE-disabling features. Disable DOCTYPE/external entities before parsing untrusted XML.")
			},
		})
	}
	{
		r := &JavaObjectInputStreamRule{BaseRule: BaseRule{RuleName: "JavaObjectInputStream", RuleSetName: "security", Sev: "warning", Desc: "Detects ObjectInputStream construction in production source."}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			NodeTypes:  []string{"call_expression", "object_creation_expression"},
			Languages:  []scanner.Language{scanner.LangKotlin, scanner.LangJava},
			Confidence: r.Confidence(), Implementation: r,
			Check: func(ctx *api.Context) {
				idx, file := ctx.Idx, ctx.File
				if !javaObjectInputStreamConstructor(file, idx) {
					return
				}
				ctx.EmitAt(file.FlatRow(idx)+1, file.FlatCol(idx)+1, "ObjectInputStream enables Java deserialization gadget attacks. Prefer JSON, protobuf, or kotlinx.serialization for untrusted data.")
			},
		})
	}
	{
		r := &JacksonDefaultTypingRule{BaseRule: BaseRule{RuleName: "JacksonDefaultTyping", RuleSetName: "security", Sev: "warning", Desc: "Detects Jackson default typing APIs that enable polymorphic deserialization."}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			NodeTypes:  []string{"call_expression", "method_invocation"},
			Languages:  []scanner.Language{scanner.LangKotlin, scanner.LangJava},
			Confidence: r.Confidence(), Implementation: r,
			Check: func(ctx *api.Context) {
				idx, file := ctx.Idx, ctx.File
				if !jacksonDefaultTypingCall(file, idx) {
					return
				}
				ctx.EmitAt(file.FlatRow(idx)+1, file.FlatCol(idx)+1, "Jackson default typing enables polymorphic deserialization gadget attacks. Avoid enableDefaultTyping/activateDefaultTyping for untrusted data.")
			},
		})
	}
	{
		r := &GsonPolymorphicFromJSONRule{BaseRule: BaseRule{RuleName: "GsonPolymorphicFromJson", RuleSetName: "security", Sev: "warning", Desc: "Detects Gson fromJson calls that deserialize into Object/Any."}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			NodeTypes:  []string{"call_expression", "method_invocation"},
			Languages:  []scanner.Language{scanner.LangKotlin, scanner.LangJava},
			Confidence: r.Confidence(), Implementation: r,
			Check: func(ctx *api.Context) {
				idx, file := ctx.Idx, ctx.File
				if !gsonPolymorphicFromJSONCall(file, idx) {
					return
				}
				ctx.EmitAt(file.FlatRow(idx)+1, file.FlatCol(idx)+1, "Gson fromJson deserializes into Object/Any. Deserialize into a concrete DTO type instead.")
			},
		})
	}
	{
		r := &FileFromUntrustedPathRule{BaseRule: BaseRule{RuleName: "FileFromUntrustedPath", RuleSetName: "security", Sev: "info", Desc: "Detects File construction from untrusted input in extraction or download functions without path traversal guards."}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			NodeTypes: []string{"call_expression"}, Confidence: api.ConfidenceMedium, Implementation: r,
			Check: func(ctx *api.Context) {
				idx, file := ctx.Idx, ctx.File
				if flatCallExpressionName(file, idx) != "File" {
					return
				}
				fn, ok := flatEnclosingFunction(file, idx)
				if !ok {
					return
				}
				fnName := strings.ToLower(extractIdentifierFlat(file, fn))
				if !isRiskyFileFromPathFunction(fnName) {
					return
				}
				_, args := flatCallExpressionParts(file, idx)
				if args == 0 {
					return
				}
				parentArg := flatPositionalValueArgument(file, args, 0)
				childArg := flatPositionalValueArgument(file, args, 1)
				if parentArg == 0 || childArg == 0 {
					return
				}
				parentExpr := valueArgumentExpressionTextFlat(file, parentArg)
				childExpr := valueArgumentExpressionTextFlat(file, childArg)
				if childExpr == "" {
					return
				}
				if isStringLiteralExpr(childExpr) {
					if !strings.Contains(childExpr, "..") {
						return
					}
				} else if hasCanonicalPathContainmentGuardFlat(file, fn, parentExpr) {
					return
				}
				ctx.EmitAt(file.FlatRow(childArg)+1, file.FlatCol(childArg)+1, "File child path comes from untrusted input in extraction/download code. Reject '..' segments or enforce canonical-path containment before writing.")
			},
		})
	}
	{
		r := &ZipSlipUncheckedRule{BaseRule: BaseRule{RuleName: "ZipSlipUnchecked", RuleSetName: "security", Sev: "info", Desc: "Detects zip extraction loops that build a destination File from an entry name without a canonical-path containment check."}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			NodeTypes: []string{"call_expression"}, Confidence: api.ConfidenceMedium, Implementation: r,
			Check: func(ctx *api.Context) {
				idx, file := ctx.Idx, ctx.File
				name := flatCallExpressionName(file, idx)
				if name != "File" && name != "resolve" {
					return
				}
				_, args := flatCallExpressionParts(file, idx)
				if args == 0 {
					return
				}
				var childArg uint32
				if name == "File" {
					childArg = flatPositionalValueArgument(file, args, 1)
				} else {
					childArg = flatPositionalValueArgument(file, args, 0)
				}
				if childArg == 0 {
					return
				}
				if !zipSlipChildArgIsEntryName(valueArgumentExpressionTextFlat(file, childArg)) {
					return
				}
				if !zipSlipInsideExtractionLoop(file, idx) {
					return
				}
				fn, ok := flatEnclosingFunction(file, idx)
				if !ok {
					return
				}
				if !zipSlipFunctionMentionsZipAPI(file, fn) {
					return
				}
				if zipSlipFunctionHasGuard(file, fn) {
					return
				}
				ctx.EmitAt(file.FlatRow(idx)+1, file.FlatCol(idx)+1, "Zip entry path written without canonical-path containment check. Verify out.canonicalPath.startsWith(destDir.canonicalPath + File.separator) or Path.normalize().startsWith(destDir.normalize()) before writing the file.")
			},
		})
	}
	{
		r := &HardcodedGcpServiceAccountRule{BaseRule: BaseRule{RuleName: "HardcodedGcpServiceAccount", RuleSetName: "security", Sev: "warning", Desc: "Detects embedded GCP service-account JSON or private keys committed into source files."}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			NodeTypes: []string{"string_literal"}, Languages: []scanner.Language{scanner.LangKotlin, scanner.LangJava}, Confidence: api.ConfidenceMedium, Implementation: r,
			Check: func(ctx *api.Context) {
				idx, file := ctx.Idx, ctx.File
				lowerPath := strings.ToLower(file.Path)
				if strings.HasSuffix(lowerPath, ".pem") || strings.HasSuffix(lowerPath, ".json") {
					return
				}
				text := file.FlatNodeText(idx)
				body, ok := kotlinStringLiteralBody(text)
				if !ok || !looksLikeHardcodedGcpServiceAccount(body) {
					return
				}
				ctx.EmitAt(file.FlatRow(idx)+1, file.FlatCol(idx)+1, "Hardcoded GCP service account credential literal. Load it from a file or secret storage instead of embedding it in source.")
			},
		})
	}
	{
		r := &HardcodedBearerTokenRule{BaseRule: BaseRule{RuleName: "HardcodedBearerToken", RuleSetName: "security", Sev: "warning", Desc: "Detects bearer authorization strings with hardcoded tokens embedded directly in source code."}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			NodeTypes: []string{"string_literal"}, Languages: []scanner.Language{scanner.LangKotlin, scanner.LangJava}, Confidence: api.ConfidenceMedium, Implementation: r,
			Check: func(ctx *api.Context) {
				idx, file := ctx.Idx, ctx.File
				text := file.FlatNodeText(idx)
				if _, ok := extractHardcodedBearerToken(text); !ok {
					return
				}
				ctx.EmitAt(file.FlatRow(idx)+1, file.FlatCol(idx)+1, "Hardcoded bearer token literal. Load the token from config or secret storage instead of embedding it in source.")
			},
		})
	}
	{
		r := &HardcodedJwtRule{BaseRule: BaseRule{RuleName: "HardcodedJwt", RuleSetName: "security", Sev: "warning", Desc: "Detects string literals that look like a JSON Web Token committed directly into source."}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			NodeTypes: []string{"string_literal"}, Languages: []scanner.Language{scanner.LangKotlin, scanner.LangJava}, Confidence: r.Confidence(), Implementation: r,
			Check: func(ctx *api.Context) {
				idx, file := ctx.Idx, ctx.File
				text := file.FlatNodeText(idx)
				body, ok := kotlinStringLiteralBody(text)
				if !ok {
					return
				}
				if !looksLikeHardcodedJwt(body) {
					return
				}
				ctx.EmitAt(file.FlatRow(idx)+1, file.FlatCol(idx)+1, "Hardcoded JWT literal in source. Load tokens from a secret store or environment, not the repository.")
			},
		})
	}
	{
		r := &HardcodedAwsAccessKeyRule{BaseRule: BaseRule{RuleName: "HardcodedAwsAccessKey", RuleSetName: "security", Sev: "warning", Desc: "Detects string literals that look like an AWS access-key ID committed directly into source."}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			NodeTypes: []string{"string_literal"}, Languages: []scanner.Language{scanner.LangKotlin, scanner.LangJava}, Confidence: r.Confidence(), Implementation: r,
			Check: func(ctx *api.Context) {
				idx, file := ctx.Idx, ctx.File
				text := file.FlatNodeText(idx)
				body, ok := kotlinStringLiteralBody(text)
				if !ok {
					return
				}
				if !looksLikeHardcodedAwsAccessKey(body) {
					return
				}
				ctx.EmitAt(file.FlatRow(idx)+1, file.FlatCol(idx)+1, "Hardcoded AWS access-key ID in source. Load credentials from environment, instance metadata, or a secret store.")
			},
		})
	}
	{
		r := &TempFileWorldReadableRule{BaseRule: BaseRule{RuleName: "TempFileWorldReadable", RuleSetName: "security", Sev: "info", Desc: "Detects setReadable/setWritable/setExecutable(true, false) on a File from createTempFile, exposing the temp file to all users."}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			NodeTypes:      []string{"call_expression", "method_invocation"},
			Languages:      []scanner.Language{scanner.LangKotlin, scanner.LangJava},
			Confidence:     r.Confidence(),
			Implementation: r,
			Check: func(ctx *api.Context) {
				idx, file := ctx.Idx, ctx.File
				receiver, args, ok := tempFileSetterAndReceiver(file, idx)
				if !ok {
					return
				}
				// arg 0 is the readable/writable/executable flag (must be true);
				// arg 1 is ownerOnly — false means "applies to everyone".
				if !tempFileSetterArgIsLiteral(file, args, 0, "true") {
					return
				}
				if !tempFileSetterArgIsLiteral(file, args, 1, "false") {
					return
				}
				if !receiverBoundToCreateTempFile(file, idx, receiver) {
					return
				}
				ctx.EmitAt(file.FlatRow(idx)+1, file.FlatCol(idx)+1, "Temp file from createTempFile is being made world-accessible. Pass ownerOnly = true (the single-arg overload) so only the owner can read/write the file.")
			},
		})
	}
}
