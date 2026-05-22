package findmy

import "testing"

func TestSplitLocationStaleness(t *testing.T) {
	tests := []struct {
		name          string
		in            string
		wantLocation  string
		wantStaleness string
	}{
		{
			name:          "standard location and staleness",
			in:            "Bellevue, WA • 2 min. ago",
			wantLocation:  "Bellevue, WA",
			wantStaleness: "2 min. ago",
		},
		{
			name:          "location only",
			in:            "Bellevue, WA",
			wantLocation:  "Bellevue, WA",
			wantStaleness: "",
		},
		{
			name:          "this mac badge with status",
			in:            "This Mac • No location found",
			wantLocation:  "No location found",
			wantStaleness: "",
		},
		{
			name:          "this iphone badge with status",
			in:            "This iPhone • No location found",
			wantLocation:  "No location found",
			wantStaleness: "",
		},
		{
			name:          "this ipad badge with real location",
			in:            "This iPad • Home",
			wantLocation:  "Home",
			wantStaleness: "",
		},
		{
			name:          "ordinary location starting with This (not a badge)",
			in:            "This Pleasant Street • Now",
			wantLocation:  "This Pleasant Street",
			wantStaleness: "Now",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotLoc, gotStale := splitLocationStaleness(tt.in)
			if gotLoc != tt.wantLocation {
				t.Errorf("location = %q, want %q", gotLoc, tt.wantLocation)
			}
			if gotStale != tt.wantStaleness {
				t.Errorf("staleness = %q, want %q", gotStale, tt.wantStaleness)
			}
		})
	}
}
