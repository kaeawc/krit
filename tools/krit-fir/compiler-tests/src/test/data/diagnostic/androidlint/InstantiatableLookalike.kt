// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
// Classes whose supertype has the simple name of an Android component but is
// a project class, not the framework one. None of them is an Android
// component, so the message ("registered as an Android component") is not
// true of them and neither FIR nor Go reports any. Go skips a supertype that
// has no constructor call (a framework component is a class) or whose simple
// name the file declares itself.
package test.lookalike

interface Service

open class Application

abstract class Activity

open class BroadcastReceiver

open class ContentProvider

open class AppCompatActivity

private class UserService : Service

class DesktopApp private constructor() : Application()

private class Screen : Activity()

class Receiver private constructor() : BroadcastReceiver()

private class Provider : ContentProvider()

class Compat private constructor() : AppCompatActivity()

private class Qualified : test.lookalike.Service
