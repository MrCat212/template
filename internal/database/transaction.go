package database

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type TxManager interface {
	Do(ctx context.Context, fn func(ctx context.Context) error) error
}

type txManager struct {
	pool *pgxpool.Pool
}

type txContextKey struct{}

func NewTxManager(pool *pgxpool.Pool) TxManager {
	return &txManager{
		pool: pool,
	}
}

func (m *txManager) Do(
	ctx context.Context,
	fn func(ctx context.Context) error,
) (err error) {
	// Если транзакция уже есть, новую не открываем.
	if txFromContext(ctx) != nil {
		return fn(ctx)
	}

	tx, err := m.pool.BeginTx(ctx, pgx.TxOptions{
		IsoLevel: pgx.ReadCommitted,
	})
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}

	txCtx := context.WithValue(ctx, txContextKey{}, tx)

	defer func() {
		if panicValue := recover(); panicValue != nil {
			_ = tx.Rollback(context.Background())
			panic(panicValue)
		}
	}()

	if err := fn(txCtx); err != nil {
		_ = tx.Rollback(context.Background())
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}

	return nil
}

func txFromContext(ctx context.Context) pgx.Tx {
	tx, _ := ctx.Value(txContextKey{}).(pgx.Tx)
	return tx
}
