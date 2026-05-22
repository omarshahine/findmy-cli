package findmy

import (
	"reflect"
	"testing"
)

type watchEventSummary struct {
	Event   string
	Name    string
	Changes map[string][2]string
	Count   int
}

func TestWatchStateDiff(t *testing.T) {
	tests := []struct {
		name     string
		kind     WatchKind
		previous []watchRecord
		incoming []watchRecord
		want     []watchEventSummary
	}{
		{
			name: "empty to first poll emits added events",
			kind: WatchPeople,
			incoming: recordsForPeople([]Person{
				{Name: "Sadie", Location: "Bellevue, WA"},
				{Name: "Omar", Location: "Seattle, WA"},
			}),
			want: []watchEventSummary{
				{Event: "added", Name: "Omar"},
				{Event: "added", Name: "Sadie"},
			},
		},
		{
			name: "same records emit no diff events",
			kind: WatchPeople,
			previous: recordsForPeople([]Person{
				{Name: "Omar", Location: "Seattle, WA", Staleness: "now"},
			}),
			incoming: recordsForPeople([]Person{
				{Name: "Omar", Location: "Seattle, WA", Staleness: "now"},
			}),
			want: nil,
		},
		{
			name: "location change emits updated event with changes",
			kind: WatchPeople,
			previous: recordsForPeople([]Person{
				{Name: "Omar", Location: "Seattle, WA"},
			}),
			incoming: recordsForPeople([]Person{
				{Name: "Omar", Location: "Bellevue, WA"},
			}),
			want: []watchEventSummary{
				{
					Event: "updated",
					Name:  "Omar",
					Changes: map[string][2]string{
						"location": {"Seattle, WA", "Bellevue, WA"},
					},
				},
			},
		},
		{
			name: "removed row emits removed event",
			kind: WatchDevices,
			previous: recordsForDevices([]Device{
				{Name: "iPhone", Location: "Home"},
			}),
			incoming: nil,
			want: []watchEventSummary{
				{Event: "removed", Name: "iPhone"},
			},
		},
		{
			name: "added row emits added event",
			kind: WatchDevices,
			previous: recordsForDevices([]Device{
				{Name: "iPhone", Location: "Home"},
			}),
			incoming: recordsForDevices([]Device{
				{Name: "iPhone", Location: "Home"},
				{Name: "AirPods", Location: "Office"},
			}),
			want: []watchEventSummary{
				{Event: "added", Name: "AirPods"},
			},
		},
		{
			name: "device battery participates in updated changes",
			kind: WatchDevices,
			previous: recordsForDevices([]Device{
				{Name: "iPhone", Location: "Home", Battery: "82%"},
			}),
			incoming: recordsForDevices([]Device{
				{Name: "iPhone", Location: "Home", Battery: "79%"},
			}),
			want: []watchEventSummary{
				{
					Event: "updated",
					Name:  "iPhone",
					Changes: map[string][2]string{
						"battery": {"82%", "79%"},
					},
				},
			},
		},
		{
			name: "multiple event ordering is grouped and alphabetical",
			kind: WatchPeople,
			previous: recordsForPeople([]Person{
				{Name: "Charlie", Location: "Old"},
				{Name: "Alice", Location: "Home"},
				{Name: "Erin", Location: "School"},
			}),
			incoming: recordsForPeople([]Person{
				{Name: "Delta", Location: "Library"},
				{Name: "Bob", Location: "Office"},
				{Name: "Charlie", Location: "New"},
			}),
			want: []watchEventSummary{
				{Event: "added", Name: "Bob"},
				{Event: "added", Name: "Delta"},
				{Event: "removed", Name: "Alice"},
				{Event: "removed", Name: "Erin"},
				{
					Event: "updated",
					Name:  "Charlie",
					Changes: map[string][2]string{
						"location": {"Old", "New"},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state := newWatchState(tt.kind)
			if tt.previous != nil {
				state.diff(tt.previous)
			}
			got := summarizeWatchEvents(state.diff(tt.incoming))
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("diff mismatch\n got: %#v\nwant: %#v", got, tt.want)
			}
		})
	}
}

func TestUnchangedEvent(t *testing.T) {
	evt := unchangedEvent(WatchPeople, 4)
	if evt.Event != "unchanged" {
		t.Fatalf("Event = %q, want unchanged", evt.Event)
	}
	if evt.Kind != WatchPeople {
		t.Fatalf("Kind = %q, want %q", evt.Kind, WatchPeople)
	}
	if evt.Count != 4 {
		t.Fatalf("Count = %d, want 4", evt.Count)
	}
	if evt.Ts.IsZero() {
		t.Fatal("Ts is zero")
	}
}

func summarizeWatchEvents(events []WatchEvent) []watchEventSummary {
	if len(events) == 0 {
		return nil
	}
	summaries := make([]watchEventSummary, 0, len(events))
	for _, evt := range events {
		summaries = append(summaries, watchEventSummary{
			Event:   evt.Event,
			Name:    eventRecordName(evt),
			Changes: evt.Changes,
			Count:   evt.Count,
		})
	}
	return summaries
}
