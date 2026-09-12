package findmy

import (
	"reflect"
	"testing"
)

func TestWindowOwnerCandidatesAcceptsMacOSDisplayNameVariant(t *testing.T) {
	got := windowOwnerCandidates("FindMy")
	want := []string{"FindMy", "Find My"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("windowOwnerCandidates(FindMy) = %#v, want %#v", got, want)
	}
}

func TestWindowOwnerCandidatesLeavesLocalizedNameUntouched(t *testing.T) {
	got := windowOwnerCandidates("Localiser")
	want := []string{"Localiser"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("windowOwnerCandidates(Localiser) = %#v, want %#v", got, want)
	}
}
