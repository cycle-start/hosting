package handler

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/mock"
)

// handlerMockDB implements core.DB for handler tests.
type handlerMockDB struct {
	mock.Mock
}

func (m *handlerMockDB) Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error) {
	args := m.Called(ctx, sql, arguments)
	return args.Get(0).(pgconn.CommandTag), args.Error(1)
}

func (m *handlerMockDB) Query(ctx context.Context, sql string, arguments ...any) (pgx.Rows, error) {
	args := m.Called(ctx, sql, arguments)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(pgx.Rows), args.Error(1)
}

func (m *handlerMockDB) QueryRow(ctx context.Context, sql string, arguments ...any) pgx.Row {
	args := m.Called(ctx, sql, arguments)
	return args.Get(0).(pgx.Row)
}
