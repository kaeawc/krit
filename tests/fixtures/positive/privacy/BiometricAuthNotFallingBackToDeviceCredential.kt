package test

fun unlock(activity: FragmentActivity, executor: Executor, callback: BiometricPrompt.AuthenticationCallback) {
    BiometricPrompt(activity, executor, callback)
        .authenticate(PromptInfo.Builder().setTitle("Unlock").build())
}
