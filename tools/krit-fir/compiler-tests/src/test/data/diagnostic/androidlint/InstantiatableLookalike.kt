// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 23, 26, 29, 32, 35, 38, 41
// Classes whose supertype has the simple name of an Android component but is
// a project class, not the framework one. None of them is an Android
// component, so the message ("registered as an Android component") is not
// true of them and FIR reports none. Go reports each, because it matches the
// direct supertype by its simple name alone.
package test.lookalike

interface Service

open class Application

abstract class Activity

open class BroadcastReceiver

open class ContentProvider

open class AppCompatActivity

// Go reports this: a Retrofit-style `Service` interface.
private class UserService : Service

// Go reports this: a desktop-style `Application` base class.
class DesktopApp private constructor() : Application()

// Go reports this.
private class Screen : Activity()

// Go reports this.
class Receiver private constructor() : BroadcastReceiver()

// Go reports this.
private class Provider : ContentProvider()

// Go reports this.
class Compat private constructor() : AppCompatActivity()

// Go reports this: the qualified name ends in `Service`.
private class Qualified : test.lookalike.Service
