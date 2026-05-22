package main

import (
	"testing"
	"time"

	"github.com/oshahine/findmy-cli/internal/findmy"
)

func TestParseWatchOptsDefaultsDiffForJSONOnly(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		wantJSON bool
		wantDiff bool
	}{
		{
			name:     "human output defaults diff off",
			args:     []string{"people"},
			wantJSON: false,
			wantDiff: false,
		},
		{
			name:     "json output defaults diff on",
			args:     []string{"people", "--json"},
			wantJSON: true,
			wantDiff: true,
		},
		{
			name:     "json output can request unchanged heartbeats",
			args:     []string{"people", "--json", "--no-diff"},
			wantJSON: true,
			wantDiff: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts, err := parseWatchOpts(tt.args)
			if err != nil {
				t.Fatalf("parseWatchOpts returned error: %v", err)
			}
			if opts.JSON != tt.wantJSON {
				t.Fatalf("JSON = %v, want %v", opts.JSON, tt.wantJSON)
			}
			if opts.Diff != tt.wantDiff {
				t.Fatalf("Diff = %v, want %v", opts.Diff, tt.wantDiff)
			}
		})
	}
}

func TestParseWatchOptsIntervalKindAndOnce(t *testing.T) {
	opts, err := parseWatchOpts([]string{"--interval=10s", "items", "--once", "--diff"})
	if err != nil {
		t.Fatalf("parseWatchOpts returned error: %v", err)
	}
	if opts.Kind != findmy.WatchItems {
		t.Fatalf("Kind = %q, want %q", opts.Kind, findmy.WatchItems)
	}
	if opts.Interval != 10*time.Second {
		t.Fatalf("Interval = %s, want 10s", opts.Interval)
	}
	if !opts.Once {
		t.Fatal("Once = false, want true")
	}
	if !opts.Diff {
		t.Fatal("Diff = false, want true")
	}
}

func TestParseWatchOptsRejectsNonPositiveInterval(t *testing.T) {
	for _, interval := range []string{"0s", "-1s"} {
		t.Run(interval, func(t *testing.T) {
			if _, err := parseWatchOpts([]string{"people", "--interval=" + interval}); err == nil {
				t.Fatal("parseWatchOpts returned nil error")
			}
		})
	}
}
