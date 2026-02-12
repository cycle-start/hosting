package activity

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/edvin/hosting/internal/model"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// ---------- Mock DB ----------

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

// ---------- Mock Rows ----------

type mockRows struct {
	callIndex int
	scanFuncs []func(dest ...any) error
	err       error
}

func newMockRows(scanFuncs ...func(dest ...any) error) *mockRows {
	return &mockRows{scanFuncs: scanFuncs}
}

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

// ---------- ListTenantsByShard ----------

func TestCoreDB_ListTenantsByShard_Success(t *testing.T) {
	db := &mockDB{}
	a := NewCoreDB(db)
	ctx := context.Background()

	shardID := "test-shard-1"
	id1, id2 := "test-tenant-1", "test-tenant-2"
	regionID := "test-region-1"
	clusterID := "test-cluster-1"
	now := time.Now().Truncate(time.Microsecond)

	rows := newMockRows(
		func(dest ...any) error {
			*(dest[0].(*string)) = id1
			*(dest[1].(*string)) = "alpha"
			*(dest[2].(*string)) = regionID
			*(dest[3].(*string)) = clusterID
			*(dest[4].(**string)) = &shardID
			*(dest[5].(*int)) = 5001
			*(dest[6].(*bool)) = false
			*(dest[7].(*string)) = model.StatusActive
			*(dest[8].(*time.Time)) = now
			*(dest[9].(*time.Time)) = now
			return nil
		},
		func(dest ...any) error {
			*(dest[0].(*string)) = id2
			*(dest[1].(*string)) = "beta"
			*(dest[2].(*string)) = regionID
			*(dest[3].(*string)) = clusterID
			*(dest[4].(**string)) = &shardID
			*(dest[5].(*int)) = 5002
			*(dest[6].(*bool)) = true
			*(dest[7].(*string)) = model.StatusPending
			*(dest[8].(*time.Time)) = now
			*(dest[9].(*time.Time)) = now
			return nil
		},
	)
	db.On("Query", ctx, mock.AnythingOfType("string"), mock.Anything).Return(rows, nil)

	result, err := a.ListTenantsByShard(ctx, shardID)
	require.NoError(t, err)
	require.Len(t, result, 2)
	assert.Equal(t, "alpha", result[0].Name)
	assert.Equal(t, "beta", result[1].Name)
	assert.Equal(t, 5001, result[0].UID)
	assert.Equal(t, 5002, result[1].UID)
	assert.Equal(t, &shardID, result[0].ShardID)
	assert.Equal(t, &shardID, result[1].ShardID)
	db.AssertExpectations(t)
}

func TestCoreDB_ListTenantsByShard_Empty(t *testing.T) {
	db := &mockDB{}
	a := NewCoreDB(db)
	ctx := context.Background()

	rows := newEmptyMockRows()
	db.On("Query", ctx, mock.AnythingOfType("string"), mock.Anything).Return(rows, nil)

	result, err := a.ListTenantsByShard(ctx, "test-shard-1")
	require.NoError(t, err)
	assert.Empty(t, result)
	db.AssertExpectations(t)
}

func TestCoreDB_ListTenantsByShard_QueryError(t *testing.T) {
	db := &mockDB{}
	a := NewCoreDB(db)
	ctx := context.Background()

	db.On("Query", ctx, mock.AnythingOfType("string"), mock.Anything).Return(nil, errors.New("connection lost"))

	result, err := a.ListTenantsByShard(ctx, "test-shard-1")
	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "list tenants by shard")
	db.AssertExpectations(t)
}

func TestCoreDB_ListTenantsByShard_RowsErr(t *testing.T) {
	db := &mockDB{}
	a := NewCoreDB(db)
	ctx := context.Background()

	rows := newEmptyMockRows()
	rows.err = errors.New("iteration failed")
	db.On("Query", ctx, mock.AnythingOfType("string"), mock.Anything).Return(rows, nil)

	_, err := a.ListTenantsByShard(ctx, "test-shard-1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "iteration failed")
	db.AssertExpectations(t)
}

// ---------- ListNodesByShard ----------

func TestCoreDB_ListNodesByShard_Success(t *testing.T) {
	db := &mockDB{}
	a := NewCoreDB(db)
	ctx := context.Background()

	shardID := "test-shard-1"
	clusterID := "test-cluster-1"
	id1, id2 := "test-node-1", "test-node-2"
	now := time.Now().Truncate(time.Microsecond)

	rows := newMockRows(
		func(dest ...any) error {
			*(dest[0].(*string)) = id1
			*(dest[1].(*string)) = clusterID
			*(dest[2].(**string)) = &shardID
			*(dest[3].(*string)) = "node-1"
			ip1 := "10.0.0.1"
			ip61 := "::1"
			*(dest[4].(**string)) = &ip1
			*(dest[5].(**string)) = &ip61
			*(dest[6].(*[]string)) = []string{"web"}
			*(dest[7].(*string)) = model.StatusActive
			*(dest[8].(*time.Time)) = now
			*(dest[9].(*time.Time)) = now
			return nil
		},
		func(dest ...any) error {
			*(dest[0].(*string)) = id2
			*(dest[1].(*string)) = clusterID
			*(dest[2].(**string)) = &shardID
			*(dest[3].(*string)) = "node-2"
			ip2 := "10.0.0.2"
			ip62 := "::2"
			*(dest[4].(**string)) = &ip2
			*(dest[5].(**string)) = &ip62
			*(dest[6].(*[]string)) = []string{"db"}
			*(dest[7].(*string)) = model.StatusActive
			*(dest[8].(*time.Time)) = now
			*(dest[9].(*time.Time)) = now
			return nil
		},
	)
	db.On("Query", ctx, mock.AnythingOfType("string"), mock.Anything).Return(rows, nil)

	result, err := a.ListNodesByShard(ctx, shardID)
	require.NoError(t, err)
	require.Len(t, result, 2)
	assert.Equal(t, "node-1", result[0].Hostname)
	assert.Equal(t, "node-2", result[1].Hostname)
	assert.Equal(t, &shardID, result[0].ShardID)
	assert.Equal(t, &shardID, result[1].ShardID)
	assert.Equal(t, []string{"web"}, result[0].Roles)
	assert.Equal(t, []string{"db"}, result[1].Roles)
	db.AssertExpectations(t)
}

func TestCoreDB_ListNodesByShard_Empty(t *testing.T) {
	db := &mockDB{}
	a := NewCoreDB(db)
	ctx := context.Background()

	rows := newEmptyMockRows()
	db.On("Query", ctx, mock.AnythingOfType("string"), mock.Anything).Return(rows, nil)

	result, err := a.ListNodesByShard(ctx, "test-shard-1")
	require.NoError(t, err)
	assert.Empty(t, result)
	db.AssertExpectations(t)
}

func TestCoreDB_ListNodesByShard_QueryError(t *testing.T) {
	db := &mockDB{}
	a := NewCoreDB(db)
	ctx := context.Background()

	db.On("Query", ctx, mock.AnythingOfType("string"), mock.Anything).Return(nil, errors.New("connection lost"))

	result, err := a.ListNodesByShard(ctx, "test-shard-1")
	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "list nodes by shard")
	db.AssertExpectations(t)
}

func TestCoreDB_ListNodesByShard_RowsErr(t *testing.T) {
	db := &mockDB{}
	a := NewCoreDB(db)
	ctx := context.Background()

	rows := newEmptyMockRows()
	rows.err = errors.New("iteration failed")
	db.On("Query", ctx, mock.AnythingOfType("string"), mock.Anything).Return(rows, nil)

	_, err := a.ListNodesByShard(ctx, "test-shard-1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "iteration failed")
	db.AssertExpectations(t)
}
