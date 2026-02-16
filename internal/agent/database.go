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
	"strconv"
	"strings"

	"github.com/rs/zerolog"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/edvin/hosting/internal/model"
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

	// Create the user with mysql_native_password for broad client compatibility
	// (caching_sha2_password requires SSL for first remote connection).
	createSQL := fmt.Sprintf("CREATE USER '%s'@'%%' IDENTIFIED WITH mysql_native_password BY '%s'", username, escapedPassword)
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

// ReplicationStatus holds the parsed output of SHOW REPLICA STATUS.
type ReplicationStatus struct {
	IORunning        bool   `json:"io_running"`
	SQLRunning       bool   `json:"sql_running"`
	SecondsBehind    *int   `json:"seconds_behind"`
	LastError        string `json:"last_error"`
	ExecutedGTIDSet  string `json:"executed_gtid_set"`
	RetrievedGTIDSet string `json:"retrieved_gtid_set"`
}

// ConfigureReplication sets up this node as a replica of the given primary.
func (m *DatabaseManager) ConfigureReplication(ctx context.Context, primaryHost, replUser, replPassword string) error {
	m.logger.Info().Str("primary", primaryHost).Msg("configuring replication")
	_ = m.execMySQL(ctx, "STOP REPLICA")
	if err := m.execMySQL(ctx, "RESET REPLICA ALL"); err != nil {
		return fmt.Errorf("reset replica: %w", err)
	}
	sql := fmt.Sprintf(
		`CHANGE REPLICATION SOURCE TO SOURCE_HOST='%s', SOURCE_PORT=3306, SOURCE_USER='%s', SOURCE_PASSWORD='%s', SOURCE_AUTO_POSITION=1, SOURCE_CONNECT_RETRY=10, SOURCE_RETRY_COUNT=86400, GET_SOURCE_PUBLIC_KEY=1`,
		primaryHost, replUser, replPassword,
	)
	if err := m.execMySQL(ctx, sql); err != nil {
		return fmt.Errorf("change replication source: %w", err)
	}
	if err := m.execMySQL(ctx, "START REPLICA"); err != nil {
		return fmt.Errorf("start replica: %w", err)
	}
	return nil
}

// SetReadOnly makes this MySQL instance read-only or read-write.
func (m *DatabaseManager) SetReadOnly(ctx context.Context, readOnly bool) error {
	if readOnly {
		if err := m.execMySQL(ctx, "SET GLOBAL read_only = ON"); err != nil {
			return err
		}
		return m.execMySQL(ctx, "SET GLOBAL super_read_only = ON")
	}
	if err := m.execMySQL(ctx, "SET GLOBAL super_read_only = OFF"); err != nil {
		return err
	}
	return m.execMySQL(ctx, "SET GLOBAL read_only = OFF")
}

// GetReplicationStatus returns the current replication status of this node.
func (m *DatabaseManager) GetReplicationStatus(ctx context.Context) (*ReplicationStatus, error) {
	baseArgs, err := m.mysqlArgs()
	if err != nil {
		return nil, fmt.Errorf("parse mysql DSN: %w", err)
	}
	args := append(baseArgs, "-e", "SHOW REPLICA STATUS\\G")
	cmd := exec.CommandContext(ctx, "mysql", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("show replica status: %s: %w", string(output), err)
	}
	return parseReplicaStatus(string(output)), nil
}

// StopReplication stops replication on this node.
func (m *DatabaseManager) StopReplication(ctx context.Context) error {
	m.logger.Info().Msg("stopping replication")
	return m.execMySQL(ctx, "STOP REPLICA")
}

// parseReplicaStatus parses the vertical output of SHOW REPLICA STATUS.
func parseReplicaStatus(output string) *ReplicationStatus {
	status := &ReplicationStatus{}
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		parts := strings.SplitN(line, ": ", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		val := strings.TrimSpace(parts[1])
		switch key {
		case "Replica_IO_Running":
			status.IORunning = val == "Yes"
		case "Replica_SQL_Running":
			status.SQLRunning = val == "Yes"
		case "Seconds_Behind_Source":
			if val != "NULL" && val != "" {
				n, err := strconv.Atoi(val)
				if err == nil {
					status.SecondsBehind = &n
				}
			}
		case "Last_Error":
			status.LastError = val
		case "Executed_Gtid_Set":
			status.ExecutedGTIDSet = val
		case "Retrieved_Gtid_Set":
			status.RetrievedGTIDSet = val
		}
	}
	return status
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

// SyncUserHosts rebuilds MySQL user host patterns for all users of a database
// based on the current access rules. Internal network access (internalCIDR) is
// always preserved so the hosting platform itself can reach the database.
// When no rules exist, only the internal network pattern is used (internal-only).
// When rules exist, users get the internal pattern plus each rule's CIDR pattern.
func (m *DatabaseManager) SyncUserHosts(ctx context.Context, dbName string, users []model.DatabaseUser, rules []model.DatabaseAccessRule, internalCIDR string) error {
	if err := validateName(dbName); err != nil {
		return err
	}

	// Internal network is always allowed.
	internalHost := cidrToMySQLHost(internalCIDR)

	// Determine the host patterns to use.
	hosts := []string{internalHost}
	for _, rule := range rules {
		h := cidrToMySQLHost(rule.CIDR)
		if h != internalHost {
			hosts = append(hosts, h)
		}
	}

	for _, user := range users {
		if err := validateName(user.Username); err != nil {
			return err
		}

		m.logger.Info().
			Str("database", dbName).
			Str("username", user.Username).
			Strs("hosts", hosts).
			Msg("syncing user host patterns")

		escapedPassword := strings.ReplaceAll(user.Password, "'", "\\'")
		privStr := strings.Join(user.Privileges, ", ")
		if privStr == "" {
			privStr = "ALL PRIVILEGES"
		}

		// Drop all existing user entries by scanning mysql.user for matching usernames.
		baseArgs, err := m.mysqlArgs()
		if err != nil {
			return err
		}
		scanSQL := fmt.Sprintf("SELECT Host FROM mysql.user WHERE User = '%s'", user.Username)
		scanCmd := exec.CommandContext(ctx, "mysql", append(baseArgs, "-N", "-e", scanSQL)...)
		scanOut, _ := scanCmd.CombinedOutput()
		for _, host := range strings.Split(strings.TrimSpace(string(scanOut)), "\n") {
			host = strings.TrimSpace(host)
			if host != "" {
				_ = m.execMySQL(ctx, fmt.Sprintf("DROP USER IF EXISTS '%s'@'%s'", user.Username, host))
			}
		}

		// Create user entries for each allowed host.
		for _, host := range hosts {
			createSQL := fmt.Sprintf("CREATE USER '%s'@'%s' IDENTIFIED WITH mysql_native_password BY '%s'",
				user.Username, host, escapedPassword)
			if err := m.execMySQL(ctx, createSQL); err != nil {
				return fmt.Errorf("create user %s@%s: %w", user.Username, host, err)
			}

			grantSQL := fmt.Sprintf("GRANT %s ON `%s`.* TO '%s'@'%s'", privStr, dbName, user.Username, host)
			if err := m.execMySQL(ctx, grantSQL); err != nil {
				return fmt.Errorf("grant privileges to %s@%s: %w", user.Username, host, err)
			}
		}
	}

	return m.execMySQL(ctx, "FLUSH PRIVILEGES")
}

// cidrToMySQLHost converts a CIDR notation to a MySQL host pattern.
// Examples:
//   - "10.0.0.0/8" -> "10.%.%.%"
//   - "192.168.1.0/24" -> "192.168.1.%"
//   - "10.0.0.5/32" -> "10.0.0.5"
//   - For IPv6, returns the full CIDR as MySQL supports it since 8.0.23.
func cidrToMySQLHost(cidr string) string {
	// Parse the CIDR.
	ip, ipNet, err := net.ParseCIDR(cidr)
	if err != nil {
		// If it's not a valid CIDR, return it as-is (could be a hostname or IP).
		return cidr
	}

	// IPv6: MySQL 8.0.23+ supports CIDR-like notation with netmask.
	if ip.To4() == nil {
		ones, _ := ipNet.Mask.Size()
		return fmt.Sprintf("%s/%d", ipNet.IP.String(), ones)
	}

	// IPv4: Convert to MySQL wildcard pattern.
	ones, _ := ipNet.Mask.Size()
	parts := strings.Split(ipNet.IP.String(), ".")

	switch {
	case ones == 32:
		return ip.String()
	case ones >= 24:
		return parts[0] + "." + parts[1] + "." + parts[2] + ".%"
	case ones >= 16:
		return parts[0] + "." + parts[1] + ".%.%"
	case ones >= 8:
		return parts[0] + ".%.%.%"
	default:
		return "%"
	}
}
