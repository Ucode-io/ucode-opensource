package googlecalendar

import (
	"testing"
	"time"

	pb "github.com/Ucode-io/ucode-opensource/services/gateway/genproto/company_service"
)

func TestEventFromDataMapsProjectStatusToGoogleStatus(t *testing.T) {
	event, err := eventFromData(testMappingWithStatus(), map[string]any{
		"title":      "New Meeting",
		"start_date": "2026-06-20 11:52:00",
		"end_date":   "2026-06-20 12:53:00",
		"status":     "scheduled",
	})
	if err != nil {
		t.Fatalf("eventFromData() error = %v", err)
	}
	if event.Status != "confirmed" {
		t.Fatalf("event.Status = %q, want %q", event.Status, "confirmed")
	}
}

func TestEventFromDataIgnoresUnknownProjectStatus(t *testing.T) {
	event, err := eventFromData(testMappingWithStatus(), map[string]any{
		"title":      "New Meeting",
		"start_date": "2026-06-20 11:52:00",
		"end_date":   "2026-06-20 12:53:00",
		"status":     "in-review",
	})
	if err != nil {
		t.Fatalf("eventFromData() error = %v", err)
	}
	if event.Status != "" {
		t.Fatalf("event.Status = %q, want empty status", event.Status)
	}
}

func TestEventDateTimeNormalizesTimestampWithoutTimezone(t *testing.T) {
	tests := []struct {
		name  string
		value any
		want  string
	}{
		{
			name:  "space separator",
			value: "2026-06-21 12:13:00",
			want:  "2026-06-21T12:13:00Z",
		},
		{
			name:  "T separator without offset",
			value: "2026-06-21T12:13:00",
			want:  "2026-06-21T12:13:00Z",
		},
		{
			name:  "time value",
			value: time.Date(2026, 6, 21, 12, 13, 0, 0, time.UTC),
			want:  "2026-06-21T12:13:00Z",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := eventDateTime(tc.value)
			if err != nil {
				t.Fatalf("eventDateTime() error = %v", err)
			}
			if got.DateTime != tc.want {
				t.Fatalf("DateTime = %q, want %q", got.DateTime, tc.want)
			}
		})
	}
}

func testMappingWithStatus() *pb.GoogleCalendarMapping {
	return &pb.GoogleCalendarMapping{
		TableSlug:   "meetings",
		TitleField:  "title",
		StartField:  "start_date",
		EndField:    "end_date",
		StatusField: "status",
	}
}
