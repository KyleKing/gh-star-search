package cmd

import (
	"strings"
	"testing"
	"time"
)

func TestValidateQuery(t *testing.T) {
	tests := []struct {
		name      string
		query     string
		wantErr   bool
		errSubstr string
	}{
		{
			name:      "empty string",
			query:     "",
			wantErr:   true,
			errSubstr: "at least 2 characters",
		},
		{
			name:      "single char",
			query:     "a",
			wantErr:   true,
			errSubstr: "at least 2 characters",
		},
		{
			name:    "valid query",
			query:   "go web",
			wantErr: false,
		},
		{
			name:      "structured filter language",
			query:     "language:go",
			wantErr:   true,
			errSubstr: "structured filters",
		},
		{
			name:      "structured filter stars",
			query:     "stars:>100",
			wantErr:   true,
			errSubstr: "structured filters",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateQuery(tt.query)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateQuery(%q) error = %v, wantErr %v", tt.query, err, tt.wantErr)
				return
			}

			if tt.wantErr && tt.errSubstr != "" {
				if !strings.Contains(err.Error(), tt.errSubstr) {
					t.Errorf(
						"validateQuery(%q) error = %q, want substring %q",
						tt.query,
						err.Error(),
						tt.errSubstr,
					)
				}
			}
		})
	}
}

func TestValidateQueryFlags(t *testing.T) {
	tests := []struct {
		name      string
		mode      string
		limit     int
		long      bool
		short     bool
		wantErr   bool
		errSubstr string
	}{
		{
			name:    "valid flags",
			mode:    "fuzzy",
			limit:   10,
			long:    false,
			short:   false,
			wantErr: false,
		},
		{
			name:      "invalid mode",
			mode:      "sql",
			limit:     10,
			long:      false,
			short:     false,
			wantErr:   true,
			errSubstr: "invalid mode",
		},
		{
			name:      "limit too low",
			mode:      "fuzzy",
			limit:     0,
			long:      false,
			short:     false,
			wantErr:   true,
			errSubstr: "limit must be between",
		},
		{
			name:      "limit too high",
			mode:      "fuzzy",
			limit:     100,
			long:      false,
			short:     false,
			wantErr:   true,
			errSubstr: "limit must be between",
		},
		{
			name:      "both long and short",
			mode:      "fuzzy",
			limit:     10,
			long:      true,
			short:     true,
			wantErr:   true,
			errSubstr: "cannot specify both",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateQueryFlags(tt.mode, tt.limit, tt.long, tt.short)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateQueryFlags(%q, %d, %v, %v) error = %v, wantErr %v",
					tt.mode, tt.limit, tt.long, tt.short, err, tt.wantErr)
				return
			}

			if tt.wantErr && tt.errSubstr != "" {
				if !strings.Contains(err.Error(), tt.errSubstr) {
					t.Errorf(
						"validateQueryFlags() error = %q, want substring %q",
						err.Error(),
						tt.errSubstr,
					)
				}
			}
		})
	}
}

func TestFormatAge(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name      string
		timestamp time.Time
		want      string
	}{
		{
			name:      "zero time",
			timestamp: time.Time{},
			want:      "unknown",
		},
		{
			name:      "today",
			timestamp: now,
			want:      "today",
		},
		{
			name:      "1 day ago",
			timestamp: now.AddDate(0, 0, -1),
			want:      "1 day ago",
		},
		{
			name:      "5 days ago",
			timestamp: now.AddDate(0, 0, -5),
			want:      "5 days ago",
		},
		{
			name:      "14 days ago",
			timestamp: now.AddDate(0, 0, -14),
			want:      "2 weeks ago",
		},
		{
			name:      "45 days ago",
			timestamp: now.AddDate(0, 0, -45),
			want:      "1 month ago",
		},
		{
			name:      "400 days ago",
			timestamp: now.AddDate(0, 0, -400),
			want:      "1 year ago",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatAge(tt.timestamp)
			if got != tt.want {
				t.Errorf("formatAge() = %q, want %q", got, tt.want)
			}
		})
	}
}
