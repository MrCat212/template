package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type TxManager interface {
	Do(ctx context.Context, fn func(ctx context.Context) error) error
}

type txManager struct {
	pool    *pgxpool.Pool
	timeout time.Duration
}

type txContextKey struct{}

func NewTxManager(pool *pgxpool.Pool, timeout time.Duration) TxManager {
	return &txManager{pool: pool, timeout: timeout}
}

func (m *txManager) Do(ctx context.Context, fn func(ctx context.Context) error) error {
	if txFromContext(ctx) != nil {
		return fn(ctx)
	}

	boundedCtx, cancel := context.WithTimeout(ctx, m.timeout)
	defer cancel()

	tx, err := m.pool.BeginTx(boundedCtx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}

	committed := false
	defer func() {
		if committed {
			return
		}
		rollbackCtx, rollbackCancel := context.WithTimeout(context.Background(), m.timeout)
		defer rollbackCancel()
		_ = tx.Rollback(rollbackCtx)
	}()

	transactionCtx := context.WithValue(boundedCtx, txContextKey{}, tx)
	if err := fn(transactionCtx); err != nil {
		return err
	}

	if err := tx.Commit(boundedCtx); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}
	committed = true
	return nil
}

func txFromContext(ctx context.Context) pgx.Tx {
	tx, _ := ctx.Value(txContextKey{}).(pgx.Tx)
	return tx
}
