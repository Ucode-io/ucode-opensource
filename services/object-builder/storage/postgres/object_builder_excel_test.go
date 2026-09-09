package postgres

import "testing"

func TestFormatExcelDateValue(t *testing.T) {
	tests := []struct {
		name    string
		value   any
		want    string
		wantErr bool
	}{
		{
			name:  "empty string",
			value: "",
			want:  "",
		},
		{
			name:  "nil value",
			value: nil,
			want:  "",
		},
		{
			name:  "date only",
			value: "2026-07-02",
			want:  "02.07.2026",
		},
		{
			name:  "date with time",
			value: "2026-07-02 13:45:00",
			want:  "02.07.2026",
		},
		{
			name:    "malformed date",
			value:   "not-a-date",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := formatExcelDateValue(tt.value)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("got %q, want %q", got, tt.want)
			}
		})
	}
}
