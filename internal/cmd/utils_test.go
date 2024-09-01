package cmd

import (
	"database/sql"
	"testing"
)

func TestFormatLength(t *testing.T) {

	noneTests := []struct {
		name  string
		want  string
		value sql.NullString
	}{
		{"empty value", "null", sql.NullString{Valid: false}},
		{"zero value", "0", sql.NullString{Valid: true}},
		{"actual value", "5", sql.NullString{Valid: true, String: "12345"}},
	}

	for _, tt := range noneTests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatLength(tt.value)
			if tt.want != got {
				t.Errorf("Unexpected output, want %s, got %s", tt.want, got)
			}
		})
	}
}

func TestFormatBytes(t *testing.T) {
	sizeFormatTests := []struct {
		name  string
		want  string
		value int
	}{
		{"byte", "3", 3},
		{"KiB", "3.50 KiB", 3584},
		{"MiB", "3.50 MiB", 3670016},
		{"GiB", "3.50 GiB", 3758096384},
	}
	for _, tt := range sizeFormatTests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatBytes(tt.value)
			if tt.want != got {
				t.Errorf("Unexpected output, want %s, got %s", tt.want, got)
			}
		})
	}
}
