package dev.jasonpearson.krit.fir.support

/** Annotations after which a framework initialises or reads a declaration (Go's `frameworkAnnotationNames`). */
val frameworkInjectedAnnotations: Set<String> = setOf(
    "Mock", "MockK", "RelaxedMockK", "SpyK", "InjectMocks", "Inject", "Autowired", "BindView", "BindViews",
    "Captor", "Spy", "Value", "EJB", "Resource", "PersistenceContext",
)

/** Quoted `@Suppress` arguments that Go reads as "this unused-member finding is waived". */
val unusedMemberSuppressions: List<String> = listOf(
    "\"unused\"", "\"UNUSED_PARAMETER\"", "\"UNUSED_VARIABLE\"", "\"UnusedPrivateProperty\"",
    "\"UnusedPrivateMember\"", "\"UnusedPrivateFunction\"", "\"UnusedVariable\"",
)
