package store

import "testing"

func TestMigrationVersion(t *testing.T) {
	tests := []struct {
		name    string
		file    string
		want    int
		wantErr bool
	}{
		{"single digit", "0001_init.sql", 1, false},
		{"multi digit", "0012_add_foo.sql", 12, false},
		{"no separator", "init.sql", 0, true},
		{"non-numeric prefix", "abcd_init.sql", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := migrationVersion(tt.file)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("migrationVersion(%q) = nil error, want error", tt.file)
				}
				return
			}
			if err != nil {
				t.Fatalf("migrationVersion(%q) unexpected error: %v", tt.file, err)
			}
			if got != tt.want {
				t.Errorf("migrationVersion(%q) = %d, want %d", tt.file, got, tt.want)
			}
		})
	}
}
