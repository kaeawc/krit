package rules

import (
	"fmt"
	"github.com/kaeawc/krit/internal/oracle"
	v2 "github.com/kaeawc/krit/internal/rules/v2"
	"github.com/kaeawc/krit/internal/scanner"
	"github.com/kaeawc/krit/internal/typeinfer"
	"regexp"
	"strings"
)

func registerCoroutinesRules() {

	// --- from coroutines.go ---
	{
		r := &CollectInOnCreateWithoutLifecycleRule{BaseRule: BaseRule{RuleName: "CollectInOnCreateWithoutLifecycle", RuleSetName: "coroutines", Sev: "warning", Desc: "Detects Flow.collect calls in lifecycle callbacks that are not wrapped by repeatOnLifecycle."}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"call_expression"}, Confidence: 0.75, OriginalV1: r,
			Check: func(ctx *v2.Context) {
				idx, file := ctx.Idx, ctx.File
				navExpr, _ := flatCallExpressionParts(file, idx)
				if navExpr == 0 || flatNavigationExpressionLastIdentifier(file, navExpr) != "collect" {
					return
				}
				fn, ok := flatEnclosingFunction(file, idx)
				if !ok || !lifecycleCollectCallbacks[extractIdentifierFlat(file, fn)] {
					return
				}
				if hasAncestorCallNamedFlat(file, idx, "repeatOnLifecycle") {
					return
				}
				ctx.EmitAt(file.FlatRow(idx)+1, file.FlatCol(idx)+1,
					"Flow.collect inside onCreate/onStart/onViewCreated should be wrapped in repeatOnLifecycle to stop collecting when the lifecycle is stopped.")
			},
		})
	}
	{
		r := &GlobalCoroutineUsageRule{BaseRule: BaseRule{RuleName: "GlobalCoroutineUsage", RuleSetName: "coroutines", Sev: "warning", Desc: "Detects GlobalScope.launch/async usage instead of structured concurrency with a proper CoroutineScope."}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"call_expression", "navigation_expression"}, Confidence: 0.75, Fix: v2.FixSemantic, OriginalV1: r,
			Check: func(ctx *v2.Context) {
				idx, file := ctx.Idx, ctx.File
				switch file.FlatType(idx) {
				case "call_expression":
					if file.FlatChildCount(idx) == 0 {
						return
					}
					nav := file.FlatChild(idx, 0)
					if file.FlatType(nav) != "navigation_expression" {
						return
					}
					if file.FlatChildCount(nav) < 2 {
						return
					}
					receiver := file.FlatNodeText(file.FlatChild(nav, 0))
					if receiver != "GlobalScope" {
						return
					}
					navSuffix := file.FlatChild(nav, file.FlatChildCount(nav)-1)
					callee := ""
					if file.FlatType(navSuffix) == "navigation_suffix" {
						for j := 0; j < file.FlatChildCount(navSuffix); j++ {
							if child := file.FlatChild(navSuffix, j); file.FlatType(child) == "simple_identifier" {
								callee = file.FlatNodeText(child)
								break
							}
						}
					} else {
						callee = file.FlatNodeText(navSuffix)
					}
					if callee != "launch" && callee != "async" {
						return
					}
					f := r.Finding(file, file.FlatRow(idx)+1, file.FlatCol(idx)+1,
						"GlobalScope usage detected. Prefer structured concurrency with a proper CoroutineScope.")
					recvStart := int(file.FlatStartByte(file.FlatChild(nav, 0)))
					calleeStart := int(file.FlatStartByte(navSuffix)) + 1
					if file.FlatType(navSuffix) == "navigation_suffix" {
						for j := 0; j < file.FlatChildCount(navSuffix); j++ {
							if child := file.FlatChild(navSuffix, j); file.FlatType(child) == "simple_identifier" {
								calleeStart = int(file.FlatStartByte(child))
								break
							}
						}
					}
					f.Fix = &scanner.Fix{
						ByteMode:    true,
						StartByte:   recvStart,
						EndByte:     calleeStart,
						Replacement: "",
					}
					ctx.Emit(f)
				case "navigation_expression":
					parent, ok := file.FlatParent(idx)
					if ok && file.FlatType(parent) == "call_expression" && file.FlatChild(parent, 0) == idx {
						return
					}
					if file.FlatChildCount(idx) < 2 {
						return
					}
					receiver := file.FlatNodeText(file.FlatChild(idx, 0))
					if receiver != "GlobalScope" {
						return
					}
					ctx.EmitAt(file.FlatRow(idx)+1, file.FlatCol(idx)+1,
						"GlobalScope usage detected. Prefer structured concurrency with a proper CoroutineScope.")
				}
			},
		})
	}
	{
		r := &InjectDispatcherRule{BaseRule: BaseRule{RuleName: "InjectDispatcher", RuleSetName: "coroutines", Sev: "warning", Desc: "Detects hardcoded Dispatchers.IO/Default/Unconfined passed as arguments instead of injected dispatchers."}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"call_expression"}, Confidence: 0.75, OriginalV1: r,
			Check: func(ctx *v2.Context) {
				idx, file := ctx.Idx, ctx.File
				if isTestFile(file.Path) {
					return
				}
				if first := file.FlatChild(idx, 0); first != 0 && file.FlatType(first) == "call_expression" {
					return
				}
				args := directCallArgumentsFlat(file, idx)
				if args == 0 {
					return
				}
				dispatcherNode, dispatcherName := findDirectDispatcherArgumentFlat(file, args, injectDispatcherNames(r.DispatcherNames))
				if dispatcherNode == 0 {
					return
				}
				if !injectDispatcherReferenceConfirmed(file, dispatcherNode) {
					return
				}
				matchLine := file.FlatRow(dispatcherNode) + 1
				matchCol := file.FlatCol(dispatcherNode) + 1
				ctx.EmitAt(matchLine, matchCol,
					fmt.Sprintf("Hardcoded Dispatchers.%s. Inject dispatchers for better testability.", dispatcherName))
			},
		})
	}
	{
		r := &RedundantSuspendModifierRule{BaseRule: BaseRule{RuleName: "RedundantSuspendModifier", RuleSetName: "coroutines", Sev: "warning", Desc: "Detects suspend functions that contain no suspend calls in their body."}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"function_declaration"}, Confidence: 0.75, Fix: v2.FixIdiomatic, OriginalV1: r,
			Needs:  v2.NeedsTypeInfo,
			Oracle: &v2.OracleFilter{Identifiers: []string{"suspend"}},
			OracleCallTargets: &v2.OracleCallTargetFilter{
				CalleeNames:          redundantSuspendCallTargetCallees(),
				LexicalHintsByCallee: redundantSuspendCallTargetLexicalHints(),
			},
			// Uses expression-level call metadata only; never walks the
			// declarations map.
			OracleDeclarationNeeds: &v2.OracleDeclarationProfile{},
			Check: func(ctx *v2.Context) {
				idx, file := ctx.Idx, ctx.File
				var oracleLookup oracle.Lookup
				if cr, ok := ctx.Resolver.(*oracle.CompositeResolver); ok {
					oracleLookup = cr.Oracle()
				}
				if !hasSuspendModifierFlat(file, idx) {
					return
				}
				if file.FlatHasModifier(idx, "open") ||
					file.FlatHasModifier(idx, "abstract") ||
					file.FlatHasModifier(idx, "override") {
					return
				}
				for p, ok := file.FlatParent(idx); ok; p, ok = file.FlatParent(p) {
					if file.FlatType(p) == "class_declaration" {
						for i := 0; i < file.FlatChildCount(p); i++ {
							c := file.FlatChild(p, i)
							if file.FlatType(c) == "interface" {
								return
							}
						}
						break
					}
				}
				body, _ := file.FlatFindChild(idx, "function_body")
				if body == 0 {
					return
				}
				hasSuspendCall := false
				hasUnresolvedCall := false
				file.FlatWalkNodes(body, "call_expression", func(callIdx uint32) {
					if hasSuspendCall {
						return
					}
					provenNonSuspend := false
					if oracleLookup != nil {
						if isSuspend, ok := oracleLookupCallTargetSuspendFlat(oracleLookup, file, callIdx); ok {
							if isSuspend {
								hasSuspendCall = true
								return
							}
							provenNonSuspend = true
						}
						if ct := oracleLookupCallTargetFlat(oracleLookup, file, callIdx); ct != "" {
							if knownSuspendFQNs[ct] {
								hasSuspendCall = true
								return
							}
							for _, prefix := range suspendFQNPrefixes {
								if strings.HasPrefix(ct, prefix) {
									hasSuspendCall = true
									return
								}
							}
						}
					}
					callText := file.FlatNodeText(callIdx)
					for name := range knownSuspendFunctions {
						if strings.HasPrefix(callText, name+"(") || strings.HasPrefix(callText, name+" ") ||
							strings.HasPrefix(callText, name+"{") || strings.HasPrefix(callText, name+"<") ||
							strings.Contains(callText, "."+name+"(") {
							hasSuspendCall = true
							return
						}
					}
					if ctx.Resolver != nil {
						resolvedType := ctx.Resolver.ResolveFlatNode(callIdx, file)
						if resolvedType.Kind != typeinfer.TypeUnknown {
							callName := resolvedType.Name
							if callName != "" && knownSuspendFunctions[callName] {
								hasSuspendCall = true
								return
							}
						}
						funcIdent, _ := file.FlatFindChild(callIdx, "simple_identifier")
						if funcIdent != 0 {
							funcName := file.FlatNodeText(funcIdent)
							resolvedByName := ctx.Resolver.ResolveByNameFlat(funcName, funcIdent, file)
							if resolvedByName != nil && resolvedByName.Kind != typeinfer.TypeUnknown {
								if strings.Contains(resolvedByName.FQN, "kotlinx.coroutines") {
									hasSuspendCall = true
									return
								}
							}
						}
					}
					if !provenNonSuspend && file.FlatChildCount(callIdx) > 0 {
						first := file.FlatChild(callIdx, 0)
						if first != 0 {
							ft := file.FlatType(first)
							if ft == "navigation_expression" || ft == "simple_identifier" {
								calleeName := file.FlatNodeText(first)
								if dot := strings.LastIndex(calleeName, "."); dot >= 0 {
									calleeName = calleeName[dot+1:]
								}
								if !commonNonSuspendCallees[calleeName] {
									hasUnresolvedCall = true
								}
							}
						}
					}
				})
				if !hasSuspendCall && hasUnresolvedCall {
					return
				}
				if !hasSuspendCall {
					name := extractIdentifierFlat(file, idx)
					f := r.Finding(file, file.FlatRow(idx)+1, 1,
						fmt.Sprintf("Function '%s' has a redundant suspend modifier. No suspend calls found inside.", name))
					suspendNode := file.FlatFindModifierNode(idx, "suspend")
					if suspendNode != 0 {
						endByte := int(file.FlatEndByte(suspendNode))
						if endByte < len(file.Content) && file.Content[endByte] == ' ' {
							endByte++
						}
						f.Fix = &scanner.Fix{
							ByteMode:    true,
							StartByte:   int(file.FlatStartByte(suspendNode)),
							EndByte:     endByte,
							Replacement: "",
						}
					}
					ctx.Emit(f)
				}
			},
		})
	}
	{
		r := &SleepInsteadOfDelayRule{BaseRule: BaseRule{RuleName: "SleepInsteadOfDelay", RuleSetName: "coroutines", Sev: "warning", Desc: "Detects Thread.sleep() usage inside suspend functions or coroutine builder lambdas instead of delay()."}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"call_expression"}, Confidence: 0.75, Fix: v2.FixIdiomatic, OriginalV1: r,
			Check: func(ctx *v2.Context) {
				idx, file := ctx.Idx, ctx.File
				if file.FlatChildCount(idx) == 0 {
					return
				}
				callee := file.FlatChild(idx, 0)
				if file.FlatType(callee) != "navigation_expression" {
					return
				}
				navText := file.FlatNodeText(callee)
				if !strings.HasPrefix(navText, "Thread") || !strings.HasSuffix(navText, "sleep") {
					return
				}
				parts := strings.SplitN(navText, ".", 2)
				if len(parts) != 2 || strings.TrimSpace(parts[0]) != "Thread" || strings.TrimSpace(parts[1]) != "sleep" {
					return
				}
				if !isInsideSuspendContextFlat(file, idx) {
					return
				}
				f := r.Finding(file, file.FlatRow(idx)+1, file.FlatCol(idx)+1,
					"Thread.sleep() used in suspend context. Use delay() instead.")
				startByte := int(file.FlatStartByte(callee))
				endByte := int(file.FlatEndByte(callee))
				if file.FlatChildCount(idx) > 1 {
					suffix := file.FlatChild(idx, 1)
					suffixText := file.FlatNodeText(suffix)
					if strings.HasPrefix(suffixText, "(") {
						endByte = int(file.FlatStartByte(suffix)) + 1
					}
				}
				f.Fix = &scanner.Fix{
					ByteMode:    true,
					StartByte:   startByte,
					EndByte:     endByte,
					Replacement: "delay(",
				}
				ctx.Emit(f)
			},
		})
	}
	{
		r := &SuspendFunWithFlowReturnTypeRule{BaseRule: BaseRule{RuleName: "SuspendFunWithFlowReturnType", RuleSetName: "coroutines", Sev: "warning", Desc: "Detects suspend functions that return a Flow type, since Flow builders are cold and do not require suspend."}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"function_declaration"}, Confidence: 0.85, Fix: v2.FixIdiomatic, OriginalV1: r,
			Check: func(ctx *v2.Context) {
				idx, file := ctx.Idx, ctx.File
				if !hasSuspendModifierFlat(file, idx) {
					return
				}
				hasFlowReturn := false
				if userType, ok := file.FlatFindChild(idx, "user_type"); ok {
					if typeIdent, ok := file.FlatFindChild(userType, "type_identifier"); ok {
						if flowTypeNames[file.FlatNodeText(typeIdent)] {
							hasFlowReturn = true
						}
					}
				}
				if !hasFlowReturn {
					return
				}
				f := r.Finding(file, file.FlatRow(idx)+1, 1,
					"Suspend function returns a Flow type. A function that returns Flow should not be suspend. The flow builder is cold and does not require a coroutine.")
				suspendNode := file.FlatFindModifierNode(idx, "suspend")
				if suspendNode != 0 {
					endByte := int(file.FlatEndByte(suspendNode))
					if endByte < len(file.Content) && file.Content[endByte] == ' ' {
						endByte++
					}
					f.Fix = &scanner.Fix{
						ByteMode:    true,
						StartByte:   int(file.FlatStartByte(suspendNode)),
						EndByte:     endByte,
						Replacement: "",
					}
				}
				ctx.Emit(f)
			},
		})
	}
	{
		r := &CoroutineLaunchedInTestWithoutRunTestRule{BaseRule: BaseRule{RuleName: "CoroutineLaunchedInTestWithoutRunTest", RuleSetName: "coroutines", Sev: "warning", Desc: "Detects launch/async calls in @Test functions that are not wrapped in runTest."}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"function_declaration"}, Confidence: 0.75, OriginalV1: r,
			Check: func(ctx *v2.Context) {
				idx, file := ctx.Idx, ctx.File
				if !hasAnnotationFlat(file, idx, "Test") {
					return
				}
				funText := file.FlatNodeText(idx)
				if strings.Contains(funText, "runTest") {
					return
				}
				file.FlatWalkNodes(idx, "call_expression", func(callNode uint32) {
					callText := file.FlatNodeText(callNode)
					if strings.HasPrefix(callText, "launch") || strings.HasPrefix(callText, "async") {
						ctx.EmitAt(file.FlatRow(callNode)+1, file.FlatCol(callNode)+1,
							"Coroutine launched in @Test without runTest. Use runTest { } to properly handle coroutines in tests.")
					}
				})
			},
		})
	}
	{
		r := &SuspendFunInFinallySectionRule{BaseRule: BaseRule{RuleName: "SuspendFunInFinallySection", RuleSetName: "coroutines", Sev: "warning", Desc: "Detects suspend function calls inside finally blocks that may not execute if the coroutine is cancelled."}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"finally_block"}, Confidence: 0.75, OriginalV1: r,
			Check: func(ctx *v2.Context) {
				idx, file := ctx.Idx, ctx.File
				file.FlatWalkNodes(idx, "call_expression", func(callNode uint32) {
					callText := file.FlatNodeText(callNode)
					for name := range knownSuspendFunctions {
						if strings.HasPrefix(callText, name+"(") || strings.HasPrefix(callText, name+" ") ||
							strings.HasPrefix(callText, name+"{") {
							ctx.EmitAt(file.FlatRow(callNode)+1, file.FlatCol(callNode)+1,
								fmt.Sprintf("Suspend function '%s' called in finally block. This may not execute if the coroutine is cancelled.", name))
							return
						}
					}
				})
			},
		})
	}
	{
		r := &SuspendFunSwallowedCancellationRule{BaseRule: BaseRule{RuleName: "SuspendFunSwallowedCancellation", RuleSetName: "coroutines", Sev: "warning", Desc: "Detects catch blocks that catch CancellationException without rethrowing, breaking structured concurrency."}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"catch_block"}, Confidence: 0.75, Fix: v2.FixSemantic, OriginalV1: r,
			Needs:                  v2.NeedsTypeInfo,
			OracleDeclarationNeeds: &v2.OracleDeclarationProfile{},
			Check: func(ctx *v2.Context) {
				idx, file := ctx.Idx, ctx.File
				caughtType := extractCaughtTypeNameFlat(file, idx)
				if caughtType == "" {
					return
				}
				catchesCancellation := false
				if caughtType == "CancellationException" {
					catchesCancellation = true
				} else if ctx.Resolver != nil {
					catchesCancellation = ctx.Resolver.IsExceptionSubtype("CancellationException", caughtType)
				} else {
					catchesCancellation = typeinfer.IsSubtypeOfException("CancellationException", caughtType)
				}
				if !catchesCancellation {
					return
				}
				tryExpr := enclosingTryExpressionFlat(file, idx)
				if tryExpr == 0 || !isInsideSuspendFunctionFlat(file, tryExpr) || !tryBlockHasSuspendCallFlat(file, tryExpr, ctx.Resolver) {
					return
				}
				caughtVar := extractCaughtVarNameFlat(file, idx)
				catchText := file.FlatNodeText(idx)
				rethrowPattern := fmt.Sprintf(`\bthrow\s+%s\b`, regexp.QuoteMeta(caughtVar))
				matched, err := regexp.MatchString(rethrowPattern, catchText)
				if err != nil {
					matched = strings.Contains(catchText, "throw "+caughtVar)
				}
				if !matched {
					msg := "CancellationException is caught but not rethrown. This can break structured concurrency."
					if caughtType != "CancellationException" {
						msg = fmt.Sprintf("Catching '%s' swallows CancellationException without rethrowing. This can break structured concurrency.", caughtType)
					}
					f := r.Finding(file, file.FlatRow(idx)+1, 1, msg)
					endByte := int(file.FlatEndByte(idx))
					if endByte > 0 && file.Content[endByte-1] == '}' {
						catchLine := file.Lines[file.FlatRow(idx)]
						indent := ""
						for _, ch := range catchLine {
							if ch == ' ' || ch == '\t' {
								indent += string(ch)
							} else {
								break
							}
						}
						varName := caughtVar
						if varName == "" {
							varName = "e"
						}
						insertion := indent + "    throw " + varName + "\n" + indent
						f.Fix = &scanner.Fix{
							ByteMode:    true,
							StartByte:   endByte - 1,
							EndByte:     endByte,
							Replacement: insertion + "}",
						}
					}
					ctx.Emit(f)
				}
			},
		})
	}
	{
		r := &SuspendFunWithCoroutineScopeReceiverRule{BaseRule: BaseRule{RuleName: "SuspendFunWithCoroutineScopeReceiver", RuleSetName: "coroutines", Sev: "warning", Desc: "Detects functions that are both suspend and extension on CoroutineScope, which should be one or the other."}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"function_declaration"}, Confidence: 0.75, Fix: v2.FixIdiomatic, OriginalV1: r,
			Check: func(ctx *v2.Context) {
				idx, file := ctx.Idx, ctx.File
				if !hasSuspendModifierFlat(file, idx) {
					return
				}
				hasCoroutineScopeReceiver := false
				nodeText := file.FlatNodeText(idx)
				funIdx := strings.Index(nodeText, "fun ")
				if funIdx >= 0 {
					afterFun := nodeText[funIdx+4:]
					trimmed := strings.TrimSpace(afterFun)
					if strings.HasPrefix(trimmed, "CoroutineScope.") {
						hasCoroutineScopeReceiver = true
					}
				}
				if !hasCoroutineScopeReceiver {
					return
				}
				f := r.Finding(file, file.FlatRow(idx)+1, 1,
					"Suspend function with CoroutineScope receiver. A function should either be suspend or be an extension on CoroutineScope, not both.")
				suspendNode := file.FlatFindModifierNode(idx, "suspend")
				if suspendNode != 0 {
					endByte := int(file.FlatEndByte(suspendNode))
					if endByte < len(file.Content) && file.Content[endByte] == ' ' {
						endByte++
					}
					f.Fix = &scanner.Fix{
						ByteMode:    true,
						StartByte:   int(file.FlatStartByte(suspendNode)),
						EndByte:     endByte,
						Replacement: "",
					}
				}
				ctx.Emit(f)
			},
		})
	}
	{
		r := &ChannelReceiveWithoutCloseRule{BaseRule: BaseRule{RuleName: "ChannelReceiveWithoutClose", RuleSetName: "coroutines", Sev: "warning", Desc: "Detects Channel properties in a class that are never closed, leaking the receiver coroutine."}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"property_declaration"}, Confidence: 0.85, OriginalV1: r,
			Check: func(ctx *v2.Context) {
				idx, file := ctx.Idx, ctx.File
				// The property's initializer must be a direct call to
				// `Channel(...)`. Previously any occurrence of "Channel<"
				// or "Channel(" in the node text would trip — even in
				// comments or a type annotation like `val x: MyChannel<String>`.
				if propertyInitializerCallCalleeName(file, idx) != "Channel" {
					return
				}
				propName := extractIdentifierFlat(file, idx)
				if propName == "" {
					return
				}
				classDecl, ok := flatEnclosingAncestor(file, idx, "class_declaration")
				if !ok {
					return
				}
				if classHasCallOn(file, classDecl, propName, channelCloseNames) {
					return
				}
				ctx.EmitAt(file.FlatRow(idx)+1, file.FlatCol(idx)+1,
					fmt.Sprintf("Channel '%s' is never closed. This leaks the receiver coroutine.", propName))
			},
		})
	}
	{
		r := &CollectionsSynchronizedListIterationRule{BaseRule: BaseRule{RuleName: "CollectionsSynchronizedListIteration", RuleSetName: "coroutines", Sev: "warning", Desc: "Detects iteration over Collections.synchronized* wrappers without external synchronization."}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"for_statement"}, Confidence: 0.75, OriginalV1: r,
			Check: func(ctx *v2.Context) {
				idx, file := ctx.Idx, ctx.File
				forText := file.FlatNodeText(idx)
				hasSyncFactory := false
				for name := range synchronizedCollectionFactories {
					if strings.Contains(forText, "Collections."+name) {
						hasSyncFactory = true
						break
					}
				}
				if !hasSyncFactory {
					return
				}
				if hasAncestorCallNamedFlat(file, idx, "synchronized") {
					return
				}
				ctx.EmitAt(file.FlatRow(idx)+1, file.FlatCol(idx)+1,
					"Iterating over a Collections.synchronized* wrapper without external synchronization. The iterator is not thread-safe.")
			},
		})
	}
	{
		r := &ConcurrentModificationIterationRule{BaseRule: BaseRule{RuleName: "ConcurrentModificationIteration", RuleSetName: "coroutines", Sev: "warning", Desc: "Detects collection mutation inside for loops that causes ConcurrentModificationException."}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"for_statement"}, Confidence: 0.75, OriginalV1: r,
			Check: func(ctx *v2.Context) {
				idx, file := ctx.Idx, ctx.File
				iterableNode := uint32(0)
				for child := file.FlatFirstChild(idx); child != 0; child = file.FlatNextSib(child) {
					if file.FlatType(child) == "simple_identifier" {
						iterableNode = child
					}
					if file.FlatType(child) == "control_structure_body" || file.FlatType(child) == "statements" {
						break
					}
				}
				if iterableNode == 0 {
					return
				}
				iterableName := file.FlatNodeText(iterableNode)
				if iterableName == "" {
					return
				}
				body, _ := file.FlatFindChild(idx, "control_structure_body")
				if body == 0 {
					return
				}
				file.FlatWalkNodes(body, "call_expression", func(callIdx uint32) {
					receiver := flatReceiverNameFromCall(file, callIdx)
					method := flatCallExpressionName(file, callIdx)
					if receiver == iterableName && mutatingMethods[method] {
						ctx.EmitAt(file.FlatRow(callIdx)+1, file.FlatCol(callIdx)+1,
							fmt.Sprintf("Collection '%s' is modified while being iterated. This causes ConcurrentModificationException.", iterableName))
					}
				})
			},
		})
	}
	{
		r := &CoroutineScopeCreatedButNeverCancelledRule{BaseRule: BaseRule{RuleName: "CoroutineScopeCreatedButNeverCancelled", RuleSetName: "coroutines", Sev: "warning", Desc: "Detects CoroutineScope properties in a class that are never cancelled, leaking coroutines."}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"property_declaration"}, Confidence: 0.85, OriginalV1: r,
			Check: func(ctx *v2.Context) {
				idx, file := ctx.Idx, ctx.File
				// Initializer must be a direct `CoroutineScope(...)` call.
				// Skip properties whose text merely mentions the type in
				// an annotation or comment.
				if propertyInitializerCallCalleeName(file, idx) != "CoroutineScope" {
					return
				}
				propName := extractIdentifierFlat(file, idx)
				if propName == "" {
					return
				}
				classDecl, ok := flatEnclosingAncestor(file, idx, "class_declaration")
				if !ok {
					return
				}
				if classHasCallOn(file, classDecl, propName, coroutineScopeCancelNames) {
					return
				}
				ctx.EmitAt(file.FlatRow(idx)+1, file.FlatCol(idx)+1,
					fmt.Sprintf("CoroutineScope '%s' is created but never cancelled. This leaks coroutines.", propName))
			},
		})
	}
	{
		r := &DeferredAwaitInFinallyRule{BaseRule: BaseRule{RuleName: "DeferredAwaitInFinally", RuleSetName: "coroutines", Sev: "warning", Desc: "Detects Deferred.await() calls inside finally blocks that can throw and mask the original exception."}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"call_expression"}, Confidence: 0.75, OriginalV1: r,
			Check: func(ctx *v2.Context) {
				idx, file := ctx.Idx, ctx.File
				navExpr, _ := flatCallExpressionParts(file, idx)
				if navExpr == 0 || flatNavigationExpressionLastIdentifier(file, navExpr) != "await" {
					return
				}
				_, inFinally := flatEnclosingAncestor(file, idx, "finally_block")
				if !inFinally {
					return
				}
				if hasAncestorCallNamedFlat(file, idx, "runCatching") {
					return
				}
				ctx.EmitAt(file.FlatRow(idx)+1, file.FlatCol(idx)+1,
					"Deferred.await() in finally block can throw and mask the original exception. Wrap in runCatching.")
			},
		})
	}
	{
		r := &FlowWithoutFlowOnRule{BaseRule: BaseRule{RuleName: "FlowWithoutFlowOn", RuleSetName: "coroutines", Sev: "warning", Desc: "Detects flow chains with a terminal operator but no flowOn, risking execution on the wrong dispatcher."}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"call_expression"}, Confidence: 0.75, OriginalV1: r,
			Check: func(ctx *v2.Context) {
				idx, file := ctx.Idx, ctx.File
				navExpr, _ := flatCallExpressionParts(file, idx)
				if navExpr == 0 {
					return
				}
				terminalOp := flatNavigationExpressionLastIdentifier(file, navExpr)
				if !flowTerminalOps[terminalOp] {
					return
				}
				chainText := file.FlatNodeText(idx)
				if !strings.Contains(chainText, "flow {") && !strings.Contains(chainText, "flow{") {
					return
				}
				if strings.Contains(chainText, ".flowOn(") {
					return
				}
				ctx.EmitAt(file.FlatRow(idx)+1, file.FlatCol(idx)+1,
					"Flow chain has a terminal operator without .flowOn(). Blocking operations in the flow builder may run on the wrong dispatcher.")
			},
		})
	}
	{
		r := &SynchronizedOnStringRule{BaseRule: BaseRule{RuleName: "SynchronizedOnString", RuleSetName: "coroutines", Sev: "warning", Desc: "Detects synchronized() blocks using a string literal as the lock monitor."}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"call_expression"}, Confidence: 0.75, OriginalV1: r,
			Check: func(ctx *v2.Context) {
				idx, file := ctx.Idx, ctx.File
				if flatCallExpressionName(file, idx) != "synchronized" {
					return
				}
				args := flatCallKeyArguments(file, idx)
				if args == 0 {
					return
				}
				firstArg := flatPositionalValueArgument(file, args, 0)
				if firstArg == 0 {
					return
				}
				argExpr := flatValueArgumentExpression(file, firstArg)
				if argExpr == 0 {
					return
				}
				if file.FlatType(argExpr) == "string_literal" || file.FlatType(argExpr) == "line_string_literal" {
					ctx.EmitAt(file.FlatRow(idx)+1, file.FlatCol(idx)+1,
						"synchronized() on a string literal. Interned strings share a monitor across classloaders. Use a dedicated Any() object.")
				}
			},
		})
	}
	{
		r := &SynchronizedOnBoxedPrimitiveRule{BaseRule: BaseRule{RuleName: "SynchronizedOnBoxedPrimitive", RuleSetName: "coroutines", Sev: "warning", Desc: "Detects synchronized() blocks using a boxed primitive value as the lock monitor."}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"call_expression"}, Confidence: 0.75, OriginalV1: r,
			Check: func(ctx *v2.Context) {
				idx, file := ctx.Idx, ctx.File
				if flatCallExpressionName(file, idx) != "synchronized" {
					return
				}
				args := flatCallKeyArguments(file, idx)
				if args == 0 {
					return
				}
				firstArg := flatPositionalValueArgument(file, args, 0)
				if firstArg == 0 {
					return
				}
				argExpr := flatValueArgumentExpression(file, firstArg)
				if argExpr == 0 {
					return
				}
				argType := file.FlatType(argExpr)
				if argType == "integer_literal" || argType == "long_literal" || argType == "boolean_literal" ||
					argType == "real_literal" || argType == "character_literal" {
					ctx.EmitAt(file.FlatRow(idx)+1, file.FlatCol(idx)+1,
						"synchronized() on a boxed primitive literal. Boxed primitives have identity-equality surprises. Use a dedicated Any() object.")
					return
				}
				if argType == "simple_identifier" {
					varName := file.FlatNodeText(argExpr)
					propType := resolvePropertyTypeInScope(file, idx, varName)
					if boxedPrimitiveTypes[propType] {
						ctx.EmitAt(file.FlatRow(idx)+1, file.FlatCol(idx)+1,
							fmt.Sprintf("synchronized() on a boxed primitive (%s). Boxed primitives have identity-equality surprises. Use a dedicated Any() object.", propType))
					}
				}
			},
		})
	}
	{
		r := &SynchronizedOnNonFinalRule{BaseRule: BaseRule{RuleName: "SynchronizedOnNonFinal", RuleSetName: "coroutines", Sev: "warning", Desc: "Detects synchronized() blocks using a var property as the lock, which can change the monitor object."}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"call_expression"}, Confidence: 0.75, OriginalV1: r,
			Check: func(ctx *v2.Context) {
				idx, file := ctx.Idx, ctx.File
				if flatCallExpressionName(file, idx) != "synchronized" {
					return
				}
				args := flatCallKeyArguments(file, idx)
				if args == 0 {
					return
				}
				firstArg := flatPositionalValueArgument(file, args, 0)
				if firstArg == 0 {
					return
				}
				argExpr := flatValueArgumentExpression(file, firstArg)
				if argExpr == 0 || file.FlatType(argExpr) != "simple_identifier" {
					return
				}
				varName := file.FlatNodeText(argExpr)
				if isVarPropertyInScope(file, idx, varName) {
					ctx.EmitAt(file.FlatRow(idx)+1, file.FlatCol(idx)+1,
						fmt.Sprintf("synchronized() on non-final property '%s'. Reassignment changes the monitor object. Use val instead of var.", varName))
				}
			},
		})
	}
	{
		r := &VolatileMissingOnDclRule{BaseRule: BaseRule{RuleName: "VolatileMissingOnDcl", RuleSetName: "coroutines", Sev: "warning", Desc: "Detects double-checked locking patterns on a var property without @Volatile annotation."}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"property_declaration"}, Confidence: 0.75, OriginalV1: r,
			Check: func(ctx *v2.Context) {
				idx, file := ctx.Idx, ctx.File
				isVar := false
				for child := file.FlatFirstChild(idx); child != 0; child = file.FlatNextSib(child) {
					if file.FlatNodeTextEquals(child, "var") {
						isVar = true
						break
					}
				}
				if !isVar {
					return
				}
				propName := extractIdentifierFlat(file, idx)
				if propName == "" {
					return
				}
				propText := file.FlatNodeText(idx)
				if !strings.Contains(propText, "null") {
					return
				}
				if hasAnnotationFlat(file, idx, "Volatile") {
					return
				}
				classDecl, ok := flatEnclosingAncestor(file, idx, "class_declaration", "object_declaration")
				if !ok {
					return
				}
				classText := file.FlatNodeText(classDecl)
				if !strings.Contains(classText, "synchronized") {
					return
				}
				nullCheckPattern := propName + " == null"
				nullChecks := strings.Count(classText, nullCheckPattern)
				if nullChecks < 2 {
					return
				}
				ctx.EmitAt(file.FlatRow(idx)+1, file.FlatCol(idx)+1,
					fmt.Sprintf("Double-checked locking on '%s' without @Volatile. Add @Volatile or use 'by lazy'.", propName))
			},
		})
	}
	{
		r := &MutableStateInObjectRule{BaseRule: BaseRule{RuleName: "MutableStateInObject", RuleSetName: "coroutines", Sev: "warning", Desc: "Detects mutable var properties inside object declarations that are shared mutable state without synchronization."}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"object_declaration"}, Confidence: 0.75, OriginalV1: r,
			Check: func(ctx *v2.Context) {
				idx, file := ctx.Idx, ctx.File
				if file.FlatType(idx) == "companion_object" {
					return
				}
				file.FlatWalkNodes(idx, "property_declaration", func(propIdx uint32) {
					parent, ok := file.FlatParent(propIdx)
					if !ok {
						return
					}
					parentType := file.FlatType(parent)
					if parentType != "class_body" && parentType != "object_declaration" {
						return
					}
					if parentType == "class_body" {
						gp, ok := file.FlatParent(parent)
						if !ok || gp != idx {
							return
						}
					}
					propText := file.FlatNodeText(propIdx)
					isVar := false
					for child := file.FlatFirstChild(propIdx); child != 0; child = file.FlatNextSib(child) {
						if file.FlatNodeTextEquals(child, "var") {
							isVar = true
							break
						}
					}
					if !isVar {
						return
					}
					for typeName := range threadSafeTypes {
						if strings.Contains(propText, typeName) {
							return
						}
					}
					propName := extractIdentifierFlat(file, propIdx)
					ctx.EmitAt(file.FlatRow(propIdx)+1, file.FlatCol(propIdx)+1,
						fmt.Sprintf("Mutable 'var %s' in object declaration. Shared mutable state without synchronization is a race condition.", propName))
				})
			},
		})
	}
	{
		r := &StateFlowMutableLeakRule{BaseRule: BaseRule{RuleName: "StateFlowMutableLeak", RuleSetName: "coroutines", Sev: "warning", Desc: "Detects publicly exposed MutableStateFlow properties that should be private with a read-only StateFlow accessor."}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"property_declaration"}, Confidence: 0.75, OriginalV1: r,
			Check: func(ctx *v2.Context) {
				idx, file := ctx.Idx, ctx.File
				propText := file.FlatNodeText(idx)
				if !strings.Contains(propText, "MutableStateFlow") {
					return
				}
				if isTestFile(file.Path) || file.FlatHasAncestorOfType(idx, "function_body") {
					return
				}
				if file.FlatHasModifier(idx, "private") || file.FlatHasModifier(idx, "protected") ||
					file.FlatHasModifier(idx, "internal") {
					return
				}
				propName := extractIdentifierFlat(file, idx)
				ctx.EmitAt(file.FlatRow(idx)+1, file.FlatCol(idx)+1,
					fmt.Sprintf("MutableStateFlow '%s' is publicly exposed. Keep it private and expose as StateFlow<T>.", propName))
			},
		})
	}
	{
		r := &SharedFlowWithoutReplayRule{BaseRule: BaseRule{RuleName: "SharedFlowWithoutReplay", RuleSetName: "coroutines", Sev: "info", Desc: "Detects MutableSharedFlow() created with default configuration that has no replay or buffer capacity."}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"property_declaration"}, Confidence: 0.75, OriginalV1: r,
			Check: func(ctx *v2.Context) {
				idx, file := ctx.Idx, ctx.File
				propText := file.FlatNodeText(idx)
				if !strings.Contains(propText, "MutableSharedFlow") {
					return
				}
				if strings.Contains(propText, "MutableSharedFlow(replay") ||
					strings.Contains(propText, "MutableSharedFlow(extraBufferCapacity") ||
					strings.Contains(propText, "MutableSharedFlow(\n") {
					return
				}
				if strings.Contains(propText, "MutableSharedFlow()") ||
					(strings.Contains(propText, "MutableSharedFlow<") && strings.Contains(propText, ">()")) {
					propName := extractIdentifierFlat(file, idx)
					ctx.EmitAt(file.FlatRow(idx)+1, file.FlatCol(idx)+1,
						fmt.Sprintf("MutableSharedFlow '%s' created without replay or extraBufferCapacity. Default config is lossy.", propName))
				}
			},
		})
	}
	{
		r := &StateFlowCompareByReferenceRule{BaseRule: BaseRule{RuleName: "StateFlowCompareByReference", RuleSetName: "coroutines", Sev: "info", Desc: "Detects redundant .distinctUntilChanged() after .map{} on StateFlow, which already deduplicates by equality."}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"call_expression"}, Confidence: 0.75, OriginalV1: r,
			Check: func(ctx *v2.Context) {
				idx, file := ctx.Idx, ctx.File
				navExpr, _ := flatCallExpressionParts(file, idx)
				if navExpr == 0 || flatNavigationExpressionLastIdentifier(file, navExpr) != "distinctUntilChanged" {
					return
				}
				chainText := file.FlatNodeText(idx)
				if !strings.Contains(chainText, ".map") {
					return
				}
				if !strings.Contains(chainText, "state") && !strings.Contains(chainText, "State") &&
					!strings.Contains(chainText, "uiState") && !strings.Contains(chainText, "flow") {
					return
				}
				ctx.EmitAt(file.FlatRow(idx)+1, file.FlatCol(idx)+1,
					"Redundant .distinctUntilChanged() after .map{}. StateFlow already deduplicates by structural equality.")
			},
		})
	}
	{
		r := &GlobalScopeLaunchInViewModelRule{BaseRule: BaseRule{RuleName: "GlobalScopeLaunchInViewModel", RuleSetName: "coroutines", Sev: "warning", Desc: "Detects GlobalScope.launch/async inside ViewModel or Presenter classes instead of viewModelScope."}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"call_expression"}, Confidence: 0.75, OriginalV1: r,
			Check: func(ctx *v2.Context) {
				idx, file := ctx.Idx, ctx.File
				receiver := flatReceiverNameFromCall(file, idx)
				if receiver != "GlobalScope" {
					return
				}
				method := flatCallExpressionName(file, idx)
				if method != "launch" && method != "async" {
					return
				}
				classDecl, ok := flatEnclosingAncestor(file, idx, "class_declaration")
				if !ok {
					return
				}
				className := extractIdentifierFlat(file, classDecl)
				if !strings.HasSuffix(className, "ViewModel") && !strings.HasSuffix(className, "Presenter") {
					return
				}
				ctx.EmitAt(file.FlatRow(idx)+1, file.FlatCol(idx)+1,
					fmt.Sprintf("GlobalScope.%s in %s. Use viewModelScope instead for lifecycle-aware cancellation.", method, className))
			},
		})
	}
	{
		r := &SupervisorScopeInEventHandlerRule{BaseRule: BaseRule{RuleName: "SupervisorScopeInEventHandler", RuleSetName: "coroutines", Sev: "info", Desc: "Detects supervisorScope with a single child operation where supervisor semantics provide no benefit."}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"call_expression"}, Confidence: 0.75, OriginalV1: r,
			Check: func(ctx *v2.Context) {
				idx, file := ctx.Idx, ctx.File
				if flatCallNameAny(file, idx) != "supervisorScope" {
					return
				}
				lambda := flatCallTrailingLambda(file, idx)
				if lambda == 0 {
					return
				}
				stmts, _ := file.FlatFindChild(lambda, "statements")
				if stmts == 0 {
					return
				}
				stmtCount := 0
				for child := file.FlatFirstChild(stmts); child != 0; child = file.FlatNextSib(child) {
					if file.FlatIsNamed(child) {
						stmtCount++
					}
				}
				if stmtCount > 1 {
					return
				}
				ctx.EmitAt(file.FlatRow(idx)+1, file.FlatCol(idx)+1,
					"supervisorScope with a single child operation. Supervisor semantics are only useful with multiple concurrent children.")
			},
		})
	}
	{
		r := &WithContextInSuspendFunctionNoopRule{BaseRule: BaseRule{RuleName: "WithContextInSuspendFunctionNoop", RuleSetName: "coroutines", Sev: "info", Desc: "Detects nested withContext calls using the same dispatcher as the parent, which is redundant."}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"call_expression"}, Needs: v2.NeedsTypeInfo, Confidence: 0.75, OriginalV1: r,
			OracleCallTargets:      &v2.OracleCallTargetFilter{CalleeNames: []string{"withContext"}},
			OracleDeclarationNeeds: &v2.OracleDeclarationProfile{},
			Check: func(ctx *v2.Context) {
				idx, file := ctx.Idx, ctx.File
				if flatCallExpressionName(file, idx) != "withContext" {
					return
				}
				dispatcher := extractWithContextDispatcher(ctx, idx)
				if dispatcher == "" {
					return
				}
				skippedSelf := false
				for p, ok := file.FlatParent(idx); ok; p, ok = file.FlatParent(p) {
					if file.FlatType(p) == "function_declaration" {
						break
					}
					if file.FlatType(p) != "call_expression" {
						continue
					}
					if !skippedSelf && flatCallNameAny(file, p) == "withContext" {
						pd := extractWithContextDispatcher(ctx, p)
						if pd == dispatcher {
							skippedSelf = true
							continue
						}
					}
					if flatCallNameAny(file, p) == "withContext" {
						parentDispatcher := extractWithContextDispatcher(ctx, p)
						if parentDispatcher == dispatcher {
							ctx.EmitAt(file.FlatRow(idx)+1, file.FlatCol(idx)+1,
								fmt.Sprintf("Redundant nested withContext(%s). The parent already switches to this dispatcher.", dispatcher))
							return
						}
					}
				}
			},
		})
	}
	{
		r := &LaunchWithoutCoroutineExceptionHandlerRule{BaseRule: BaseRule{RuleName: "LaunchWithoutCoroutineExceptionHandler", RuleSetName: "coroutines", Sev: "info", Desc: "Detects launch blocks containing throw statements but no CoroutineExceptionHandler to catch uncaught exceptions."}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"call_expression"}, Confidence: 0.75, OriginalV1: r,
			Check: func(ctx *v2.Context) {
				idx, file := ctx.Idx, ctx.File
				callee := flatCallNameAny(file, idx)
				if callee != "launch" {
					return
				}
				receiver := flatReceiverNameFromCall(file, idx)
				if receiver != "GlobalScope" && receiver != "" {
					return
				}
				lambda := flatCallTrailingLambda(file, idx)
				if lambda == 0 {
					return
				}
				lambdaText := file.FlatNodeText(lambda)
				if !strings.Contains(lambdaText, "throw ") {
					return
				}
				callText := file.FlatNodeText(idx)
				if strings.Contains(callText, "CoroutineExceptionHandler") {
					return
				}
				fnDecl, ok := flatEnclosingFunction(file, idx)
				if ok {
					fnText := file.FlatNodeText(fnDecl)
					if strings.Contains(fnText, "CoroutineExceptionHandler") {
						return
					}
				}
				ctx.EmitAt(file.FlatRow(idx)+1, file.FlatCol(idx)+1,
					"launch{} contains throw but no CoroutineExceptionHandler. Uncaught exceptions will crash the app.")
			},
		})
	}
	{
		r := &MainDispatcherInLibraryCodeRule{BaseRule: BaseRule{RuleName: "MainDispatcherInLibraryCode", RuleSetName: "coroutines", Sev: "warning", Desc: "Detects Dispatchers.Main usage in library modules that lack the kotlinx-coroutines-android dependency."}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			Needs: v2.NeedsModuleIndex | v2.NeedsTypeInfo, Confidence: r.Confidence(), OriginalV1: r,
			OracleCallTargets:      &v2.OracleCallTargetFilter{CalleeNames: []string{"Main"}},
			OracleDeclarationNeeds: &v2.OracleDeclarationProfile{},
			Check:                  r.check,
		})
	}
}
