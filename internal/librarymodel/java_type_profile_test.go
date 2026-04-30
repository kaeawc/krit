package librarymodel

import "testing"

func TestJavaTypeProfile_KnownAndroidJDKTypesAndReturns(t *testing.T) {
	profile := DefaultJavaTypeProfile()
	if !profile.IsKnownFrameworkType("android.webkit.WebSettings") {
		t.Fatal("expected WebSettings to be a known Java framework type")
	}
	if !profile.IsSubtypeCandidate("java.util.concurrent.Executor", "java.util.concurrent.ExecutorService") {
		t.Fatal("expected ExecutorService to be a subtype candidate of Executor")
	}
	if got := profile.MethodReturn("android.content.SharedPreferences", "edit", 0); got != "android.content.SharedPreferences.Editor" {
		t.Fatalf("SharedPreferences.edit return = %q", got)
	}
	if got := profile.MethodReturn("String", "replace", 2); got != "java.lang.String" {
		t.Fatalf("String.replace return = %q", got)
	}
	if !profile.AnnotationImplies("androidx.annotation.CheckResult", "CheckResult") {
		t.Fatal("expected androidx CheckResult to imply CheckResult")
	}
}

func TestFactsForProfile_DisablesRoomJavaAnnotationsWhenAbsent(t *testing.T) {
	profile := ProjectProfile{
		HasGradle:                    true,
		DependencyExtractionComplete: true,
		Dependencies: []Dependency{
			{Group: "com.squareup.okhttp3", Name: "okhttp", Version: "4.12.0"},
		},
	}
	facts := FactsForProfile(profile)
	if facts.Java.AnnotationImplies("androidx.room.Dao", "RoomDao") {
		t.Fatal("Room Java annotation facts should be disabled when Gradle proves Room is absent")
	}
	if !facts.Java.AnnotationImplies("androidx.annotation.CheckResult", "CheckResult") {
		t.Fatal("non-library-gated annotation facts should remain enabled")
	}
}

func TestFactsForProfile_KeepsRoomJavaAnnotationsWhenPresentOrUnknown(t *testing.T) {
	present := FactsForProfile(ProjectProfile{
		HasGradle:                    true,
		DependencyExtractionComplete: true,
		Dependencies: []Dependency{
			{Group: "androidx.room", Name: "room-runtime", Version: "2.6.1"},
		},
	})
	if !present.Java.AnnotationImplies("androidx.room.Dao", "RoomDao") {
		t.Fatal("Room Java annotation facts should be enabled when Room is present")
	}

	unknown := DefaultFacts()
	if !unknown.Java.AnnotationImplies("androidx.room.Query", "RoomOperation") {
		t.Fatal("Room Java annotation facts should stay conservative without project facts")
	}
}
