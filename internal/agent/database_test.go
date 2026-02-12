package agent

import (
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
)

func TestValidateName_Valid(t *testing.T) {
	validNames := []string{
		"mydb",
		"my_database",
		"DB123",
		"a",
		"test_db_name",
		"CamelCase",
		"ALL_CAPS",
		"under_score_123",
	}

	for _, name := range validNames {
		t.Run(name, func(t *testing.T) {
			err := validateName(name)
			assert.NoError(t, err, "name %q should be valid", name)
		})
	}
}

func TestValidateName_Invalid(t *testing.T) {
	invalidNames := []string{
		"",                         // empty string
		"my-database",              // hyphen
		"my database",              // space
		"db.name",                  // dot
		"db;DROP TABLE",            // SQL injection semicolon
		"db' OR '1'='1",           // SQL injection quotes
		"name--comment",            // SQL comment attempt
		"../etc/passwd",            // path traversal
		"db\x00name",              // null byte
		"db`name`",                 // backtick
		"db@host",                  // at sign
		"name()",                   // parentheses
		"name*",                    // wildcard
		"name%",                    // percent
	}

	for _, name := range invalidNames {
		t.Run(name, func(t *testing.T) {
			err := validateName(name)
			assert.Error(t, err, "name %q should be invalid", name)
		})
	}
}

func TestValidatePrivilege_Valid(t *testing.T) {
	validPrivileges := []string{
		"ALL",
		"ALL PRIVILEGES",
		"SELECT",
		"INSERT",
		"UPDATE",
		"DELETE",
		"CREATE",
		"DROP",
		"ALTER",
		"INDEX",
		"REFERENCES",
		"CREATE VIEW",
		"SHOW VIEW",
		"TRIGGER",
		"EXECUTE",
		"CREATE ROUTINE",
		"ALTER ROUTINE",
		"EVENT",
		"LOCK TABLES",
		"CREATE TEMPORARY TABLES",
		// Test case-insensitive matching.
		"all",
		"select",
		"insert",
		"all privileges",
		"  SELECT  ", // trimmed whitespace
	}

	for _, priv := range validPrivileges {
		t.Run(priv, func(t *testing.T) {
			err := validatePrivilege(priv)
			assert.NoError(t, err, "privilege %q should be valid", priv)
		})
	}
}

func TestValidatePrivilege_Invalid(t *testing.T) {
	invalidPrivileges := []string{
		"",
		"INVALID",
		"SUPER",
		"GRANT OPTION",
		"FILE",
		"PROCESS",
		"RELOAD",
		"SHUTDOWN",
		"REPLICATION",
		"DROP TABLE users; --",
		"ALL; DROP DATABASE",
		"SELECT, INSERT",         // comma-separated is not a single privilege
		"random_string",
	}

	for _, priv := range invalidPrivileges {
		t.Run(priv, func(t *testing.T) {
			err := validatePrivilege(priv)
			assert.Error(t, err, "privilege %q should be invalid", priv)
		})
	}
}

func TestValidNameRegex(t *testing.T) {
	// Verify the regex itself.
	assert.True(t, validNameRe.MatchString("abc"))
	assert.True(t, validNameRe.MatchString("ABC"))
	assert.True(t, validNameRe.MatchString("a1_b2"))
	assert.True(t, validNameRe.MatchString("_underscore"))
	assert.False(t, validNameRe.MatchString(""))
	assert.False(t, validNameRe.MatchString("has-dash"))
	assert.False(t, validNameRe.MatchString("has space"))
	assert.False(t, validNameRe.MatchString("has.dot"))
}

func TestDatabaseManager_MySQLArgs_GoDriverFormat(t *testing.T) {
	cfg := Config{MySQLDSN: "root:password123@tcp(127.0.0.1:3306)/hosting"}
	mgr := NewDatabaseManager(zerolog.Nop(), cfg)

	args, err := mgr.mysqlArgs()
	assert.NoError(t, err)
	assert.Contains(t, args, "-u")
	assert.Contains(t, args, "root")
	assert.Contains(t, args, "-ppassword123")
	assert.Contains(t, args, "-h")
	assert.Contains(t, args, "127.0.0.1")
	assert.Contains(t, args, "-P")
	assert.Contains(t, args, "3306")
}

func TestDatabaseManager_MySQLArgs_URLFormat(t *testing.T) {
	cfg := Config{MySQLDSN: "mysql://admin:secret@dbhost:3307/mydb"}
	mgr := NewDatabaseManager(zerolog.Nop(), cfg)

	args, err := mgr.mysqlArgs()
	assert.NoError(t, err)
	assert.Contains(t, args, "-u")
	assert.Contains(t, args, "admin")
	assert.Contains(t, args, "-psecret")
	assert.Contains(t, args, "-h")
	assert.Contains(t, args, "dbhost")
	assert.Contains(t, args, "-P")
	assert.Contains(t, args, "3307")
}

func TestDatabaseManager_MySQLArgs_Fallback(t *testing.T) {
	cfg := Config{MySQLDSN: "some-other-format"}
	mgr := NewDatabaseManager(zerolog.Nop(), cfg)

	args, err := mgr.mysqlArgs()
	assert.NoError(t, err)
	assert.Empty(t, args)
}

func TestDatabaseManager_MySQLArgs_GoDriverNoPassword(t *testing.T) {
	cfg := Config{MySQLDSN: "root@tcp(localhost:3306)/hosting"}
	mgr := NewDatabaseManager(zerolog.Nop(), cfg)

	args, err := mgr.mysqlArgs()
	assert.NoError(t, err)
	assert.Contains(t, args, "-u")
	assert.Contains(t, args, "root")
	// No password flag should be present.
	for _, arg := range args {
		assert.NotContains(t, arg, "-p")
	}
}
