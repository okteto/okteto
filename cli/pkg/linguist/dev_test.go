package linguist

import "testing"

func TestGetSupportedLanguages(t *testing.T) {
	l := GetSupportedLanguages()
	if len(l) != 6 {
		t.Fatalf("failed to get languages: %+v", l)
	}

	if l[len(l)-1] != Unrecognized {
		t.Errorf("%s wasn't the last choice: %+v", Unrecognized, l)
	}
}
