// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 8
package jakarta.transaction

@jakarta.enterprise.context.NormalScope
annotation class TransactionScoped

<!ScopeOnParameterizedClass!>@TransactionScoped<!>
class TransactionCache<T>

@TransactionScoped
class TransactionService
