package test

fun unlock(activity: FragmentActivity, executor: Executor, callback: BiometricPrompt.AuthenticationCallback) {
    BiometricPrompt(activity, executor, callback).authenticate(
        PromptInfo.Builder()
            .setAllowedAuthenticators(BIOMETRIC_STRONG or DEVICE_CREDENTIAL)
            .setTitle("Unlock")
            .build()
    )
}
