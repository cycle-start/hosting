package agent

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/rs/zerolog"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// validNameRe matches only alphanumeric characters and underscores.
// This prevents SQL injection in database/user names.
var validNameRe = regexp.MustCompile(`^[a-zA-Z0-9_]+$`)

// DatabaseManager handles MySQL database and user operations via the mysql CLI.
type DatabaseManager struct {
	logger zerolog.Logger
	dsn    string
}

// NewDatabaseManager creates a new DatabaseManager.
func NewDatabaseManager(logger zerolog.Logger, cfg Config) *DatabaseManager {
	return &DatabaseManager{
		logger: logger.With().Str("component", "database-manager").Logger(),
		dsn:    cfg.MySQLDSN,
	}
}

// mysqlArgs parses the DSN and returns the base mysql CLI arguments for
// authentication and host connection.
func (m *DatabaseManager) mysqlArgs() ([]string, error) {
	// Expected DSN format: user:password@tcp(host:port)/dbname
	// or simpler: user:password@host:port
	// We parse it loosely to extract user, password, host, port.
	dsn := m.dsn
	var args []string

	// Try to parse as a URL-like format: mysql://user:pass@host:port/db
	// or the Go sql driver format: user:pass@tcp(host:port)/db
	if strings.Contains(dsn, "@tcp(") {
		// Go MySQL driver format: user:pass@tcp(host:port)/dbname
		parts := strings.SplitN(dsn, "@tcp(", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid mysql DSN format")
		}

		userPass := parts[0]
		hostRest := parts[1]

		// Parse user:pass
		if idx := strings.Index(userPass, ":"); idx >= 0 {
			user := userPass[:idx]
			pass := userPass[idx+1:]
			args = append(args, "-u", user)
			if pass != "" {
				args = append(args, fmt.Sprintf("-p%s", pass))
			}
		} else {
			args = append(args, "-u", userPass)
		}

		// Parse host:port)/dbname
		if idx := strings.Index(hostRest, ")"); idx >= 0 {
			hostPort := hostRest[:idx]
			host, port, err := net.SplitHostPort(hostPort)
			if err != nil {
				// Just use it as host.
				args = append(args, "-h", hostPort)
			} else {
				args = append(args, "-h", host)
				if port != "" {
					args = append(args, "-P", port)
				}
			}
		}
	} else if strings.HasPrefix(dsn, "mysql://") {
		// URL format.
		u, err := url.Parse(dsn)
		if err != nil {
			return nil, fmt.Errorf("parse mysql DSN: %w", err)
		}
		if u.User != nil {
			args = append(args, "-u", u.User.Username())
			if pass, ok := u.User.Password(); ok && pass != "" {
				args = append(args, fmt.Sprintf("-p%s", pass))
			}
		}
		host := u.Hostname()
		port := u.Port()
		if host != "" {
			args = append(args, "-h", host)
		}
		if port != "" {
			args = append(args, "-P", port)
		}
	} else {
		// Fallback: use DSN as-is with defaults.
		return []string{}, nil
	}

	return args, nil
}

// execMySQL runs a mysql CLI command with the given SQL statement.
func (m *DatabaseManager) execMySQL(ctx context.Context, sql string) error {
	baseArgs, err := m.mysqlArgs()
	if err != nil {
		return status.Errorf(codes.Internal, "parse mysql DSN: %v", err)
	}

	args := append(baseArgs, "-e", sql)
	cmd := exec.CommandContext(ctx, "mysql", args...)
	m.logger.Debug().Str("sql", sql).Msg("executing mysql command")

	if output, err := cmd.CombinedOutput(); err != nil {
		return status.Errorf(codes.Internal, "mysql command failed: %s: %v", string(output), err)
	}

	return nil
}

// validateName checks that a name contains only safe characters.
func validateName(name string) error {
	if !validNameRe.MatchString(name) {
		return status.Errorf(codes.InvalidArgument, "invalid name %q: only alphanumeric and underscore allowed", name)
	}
	return nil
}

// validatePrivilege checks that a privilege string is a known MySQL privilege.
func validatePrivilege(priv string) error {
	allowed := map[string]bool{
		"ALL":             true,
		"ALL PRIVILEGES":  true,
		"SELECT":          true,
		"INSERT":          true,
		"UPDATE":          true,
		"DELETE":          true,
		"CREATE":          true,
		"DROP":            true,
		"ALTER":           true,
		"INDEX":           true,
		"REFERENCES":      true,
		"CREATE VIEW":     true,
		"SHOW VIEW":       true,
		"TRIGGER":         true,
		"EXECUTE":         true,
		"CREATE ROUTINE":  true,
		"ALTER ROUTINE":   true,
		"EVENT":           true,
		"LOCK TABLES":     true,
		"CREATE TEMPORARY TABLES": true,
	}
	upper := strings.ToUpper(strings.TrimSpace(priv))
	if !allowed[upper] {
		return status.Errorf(codes.InvalidArgument, "invalid privilege: %q", priv)
	}
	return nil
}

// CreateDatabase creates a new MySQL database.
func (m *DatabaseManager) CreateDatabase(ctx context.Context, name string) error {
	if err := validateName(name); err != nil {
		return err
	}

	m.logger.Info().Str("database", name).Msg("creating database")

	sql := fmt.Sprintf("CREATE DATABASE IF NOT EXISTS `%s`", name)
	return m.execMySQL(ctx, sql)
}

// DeleteDatabase drops a MySQL database.
func (m *DatabaseManager) DeleteDatabase(ctx context.Context, name string) error {
	if err := validateName(name); err != nil {
		return err
	}

	m.logger.Info().Str("database", name).Msg("deleting database")

	sql := fmt.Sprintf("DROP DATABASE IF EXISTS `%s`", name)
	return m.execMySQL(ctx, sql)
}

// CreateUser creates a new MySQL user and grants privileges on the specified database.
func (m *DatabaseManager) CreateUser(ctx context.Context, dbName, username, password string, privileges []string) error {
	if err := validateName(dbName); err != nil {
		return err
	}
	if err := validateName(username); err != nil {
		return err
	}
	for _, p := range privileges {
		if err := validatePrivilege(p); err != nil {
			return err
		}
	}

	m.logger.Info().
		Str("database", dbName).
		Str("username", username).
		Strs("privileges", privileges).
		Msg("creating database user")

	// Escape single quotes in the password to prevent injection.
	escapedPassword := strings.ReplaceAll(password, "'", "\\'")

	// Drop user if it already exists (idempotent).
	dropSQL := fmt.Sprintf("DROP USER IF EXISTS '%s'@'%%'", username)
	if err := m.execMySQL(ctx, dropSQL); err != nil {
		m.logger.Warn().Err(err).Str("username", username).Msg("drop existing user failed, continuing")
	}

	// Create the user.
	createSQL := fmt.Sprintf("CREATE USER '%s'@'%%' IDENTIFIED BY '%s'", username, escapedPassword)
	if err := m.execMySQL(ctx, createSQL); err != nil {
		return err
	}

	// Grant privileges.
	privStr := strings.Join(privileges, ", ")
	if privStr == "" {
		privStr = "ALL PRIVILEGES"
	}
	grantSQL := fmt.Sprintf("GRANT %s ON `%s`.* TO '%s'@'%%'", privStr, dbName, username)
	if err := m.execMySQL(ctx, grantSQL); err != nil {
		return err
	}

	// Flush privileges.
	return m.execMySQL(ctx, "FLUSH PRIVILEGES")
}

// UpdateUser modifies an existing MySQL user's password and/or privileges.
func (m *DatabaseManager) UpdateUser(ctx context.Context, dbName, username, password string, privileges []string) error {
	if err := validateName(dbName); err != nil {
		return err
	}
	if err := validateName(username); err != nil {
		return err
	}
	for _, p := range privileges {
		if err := validatePrivilege(p); err != nil {
			return err
		}
	}

	m.logger.Info().
		Str("database", dbName).
		Str("username", username).
		Msg("updating database user")

	// Update password if provided.
	if password != "" {
		escapedPassword := strings.ReplaceAll(password, "'", "\\'")
		alterSQL := fmt.Sprintf("ALTER USER '%s'@'%%' IDENTIFIED BY '%s'", username, escapedPassword)
		if err := m.execMySQL(ctx, alterSQL); err != nil {
			return err
		}
	}

	// Revoke all existing privileges on this database.
	revokeSQL := fmt.Sprintf("REVOKE ALL PRIVILEGES ON `%s`.* FROM '%s'@'%%'", dbName, username)
	// Ignore errors on revoke since the user might not have any grants yet.
	if err := m.execMySQL(ctx, revokeSQL); err != nil {
		m.logger.Warn().Err(err).Msg("revoke privileges failed, continuing with grant")
	}

	// Grant the new privileges.
	privStr := strings.Join(privileges, ", ")
	if privStr == "" {
		privStr = "ALL PRIVILEGES"
	}
	grantSQL := fmt.Sprintf("GRANT %s ON `%s`.* TO '%s'@'%%'", privStr, dbName, username)
	if err := m.execMySQL(ctx, grantSQL); err != nil {
		return err
	}

	return m.execMySQL(ctx, "FLUSH PRIVILEGES")
}

// DumpDatabase runs mysqldump and compresses the output to a gzipped file.
func (m *DatabaseManager) DumpDatabase(ctx context.Context, name, dumpPath string) error {
	if err := validateName(name); err != nil {
		return err
	}

	m.logger.Info().Str("database", name).Str("path", dumpPath).Msg("dumping database")

	// Create parent directory.
	if err := os.MkdirAll(filepath.Dir(dumpPath), 0750); err != nil {
		return status.Errorf(codes.Internal, "create dump directory: %v", err)
	}

	baseArgs, err := m.mysqlArgs()
	if err != nil {
		return status.Errorf(codes.Internal, "parse mysql DSN: %v", err)
	}

	// Build: mysqldump {auth args} {dbname} | gzip > {dumpPath}
	dumpArgs := append(baseArgs, "--single-transaction", "--routines", "--triggers", name)
	shell := fmt.Sprintf("mysqldump %s | gzip > %s", strings.Join(quoteArgs(dumpArgs), " "), dumpPath)
	cmd := exec.CommandContext(ctx, "bash", "-c", shell)
	m.logger.Debug().Str("shell", shell).Msg("executing mysqldump")

	if output, err := cmd.CombinedOutput(); err != nil {
		return status.Errorf(codes.Internal, "mysqldump failed: %s: %v", string(output), err)
	}

	return nil
}

// ImportDatabase imports a gzipped SQL dump into a MySQL database.
func (m *DatabaseManager) ImportDatabase(ctx context.Context, name, dumpPath string) error {
	if err := validateName(name); err != nil {
		return err
	}

	m.logger.Info().Str("database", name).Str("path", dumpPath).Msg("importing database")

	baseArgs, err := m.mysqlArgs()
	if err != nil {
		return status.Errorf(codes.Internal, "parse mysql DSN: %v", err)
	}

	// Build: gunzip -c {dumpPath} | mysql {auth args} {dbname}
	importArgs := append(baseArgs, name)
	shell := fmt.Sprintf("gunzip -c %s | mysql %s", dumpPath, strings.Join(quoteArgs(importArgs), " "))
	cmd := exec.CommandContext(ctx, "bash", "-c", shell)
	m.logger.Debug().Str("shell", shell).Msg("executing mysql import")

	if output, err := cmd.CombinedOutput(); err != nil {
		return status.Errorf(codes.Internal, "mysql import failed: %s: %v", string(output), err)
	}

	return nil
}

// quoteArgs wraps each argument in single quotes for safe shell usage.
func quoteArgs(args []string) []string {
	quoted := make([]string, len(args))
	for i, a := range args {
		quoted[i] = "'" + strings.ReplaceAll(a, "'", "'\\''") + "'"
	}
	return quoted
}

// DeleteUser drops a MySQL user.
func (m *DatabaseManager) DeleteUser(ctx context.Context, dbName, username string) error {
	if err := validateName(username); err != nil {
		return err
	}

	m.logger.Info().
		Str("database", dbName).
		Str("username", username).
		Msg("deleting database user")

	sql := fmt.Sprintf("DROP USER IF EXISTS '%s'@'%%'", username)
	return m.execMySQL(ctx, sql)
}
