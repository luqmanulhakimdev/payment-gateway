package postgres

import "testing"

func TestMigrationVersion(t *testing.T) {
	version, err := migrationVersion("000001_initial_schema.up.sql")
	if err != nil || version != 1 {
		t.Fatalf("version = %d, error = %v", version, err)
	}
	for _, invalid := range []string{"initial.up.sql", "000000_invalid.up.sql", "nope_schema.up.sql"} {
		if _, err := migrationVersion(invalid); err == nil {
			t.Errorf("migrationVersion(%q) returned no error", invalid)
		}
	}
}
