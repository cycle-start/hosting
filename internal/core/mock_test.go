package core

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/mock"
)

// ---------- Mock DB ----------

// mockDB implements the DB interface for testing.
type mockDB struct {
	mock.Mock
}

func (m *mockDB) Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error) {
	args := m.Called(ctx, sql, arguments)
	return args.Get(0).(pgconn.CommandTag), args.Error(1)
}

func (m *mockDB) Query(ctx context.Context, sql string, arguments ...any) (pgx.Rows, error) {
	args := m.Called(ctx, sql, arguments)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(pgx.Rows), args.Error(1)
}

func (m *mockDB) QueryRow(ctx context.Context, sql string, arguments ...any) pgx.Row {
	args := m.Called(ctx, sql, arguments)
	return args.Get(0).(pgx.Row)
}

// ---------- Mock Row ----------

// mockRow implements pgx.Row for testing.
type mockRow struct {
	scanFunc func(dest ...any) error
}

func (m *mockRow) Scan(dest ...any) error {
	return m.scanFunc(dest...)
}

// ---------- Mock Rows ----------

// mockRows implements pgx.Rows for testing.
// It iterates through a list of scan functions, one per row.
type mockRows struct {
	callIndex int
	scanFuncs []func(dest ...any) error
	err       error
}

func newMockRows(scanFuncs ...func(dest ...any) error) *mockRows {
	return &mockRows{scanFuncs: scanFuncs}
}

// newEmptyMockRows returns a mockRows that yields zero rows.
func newEmptyMockRows() *mockRows {
	return &mockRows{}
}

func (m *mockRows) Next() bool {
	return m.callIndex < len(m.scanFuncs)
}

func (m *mockRows) Scan(dest ...any) error {
	if m.callIndex < len(m.scanFuncs) {
		fn := m.scanFuncs[m.callIndex]
		m.callIndex++
		return fn(dest...)
	}
	return nil
}

func (m *mockRows) Err() error                                   { return m.err }
func (m *mockRows) Close()                                       {}
func (m *mockRows) CommandTag() pgconn.CommandTag                 { return pgconn.CommandTag{} }
func (m *mockRows) FieldDescriptions() []pgconn.FieldDescription { return nil }
func (m *mockRows) RawValues() [][]byte                          { return nil }
func (m *mockRows) Values() ([]any, error)                       { return nil, nil }
func (m *mockRows) Conn() *pgx.Conn                              { return nil }
