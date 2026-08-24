package postgres

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.idp.xyz/idp-bento-go/application"
	bentopg "go.idp.xyz/idp-bento-go/postgres"

	"go.idp.xyz/idp-platform-go/numbering"
)

func testPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("IDP_PLATFORM_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("IDP_PLATFORM_POSTGRES_DSN is not set")
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	return pool
}

// testStore 在独立 schema 上执行迁移资产并构造存储；结束时整体清理。
func testStore(t *testing.T) (*bentopg.DB, *SequenceStore) {
	t.Helper()
	pool := testPool(t)
	ctx := context.Background()

	schema := fmt.Sprintf("platform_test_%d", time.Now().UnixNano())
	migrations, err := Migrations(schema)
	if err != nil {
		t.Fatal(err)
	}
	for _, migration := range migrations {
		if _, err := pool.Exec(ctx, migration.SQL); err != nil {
			t.Fatalf("apply %s: %v", migration.ID, err)
		}
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), "DROP SCHEMA IF EXISTS "+schema+" CASCADE")
	})

	db, err := bentopg.NewDB(pool)
	if err != nil {
		t.Fatal(err)
	}
	store, err := NewSequenceStore(db, WithSchema(schema))
	if err != nil {
		t.Fatal(err)
	}
	return db, store
}

func TestNextIsMonotonicPerKeyAndIsolatedAcrossKeys(t *testing.T) {
	db, store := testStore(t)
	ctx := context.Background()

	var got []int64
	err := db.Transactor().WithinTransaction(ctx, func(txCtx context.Context) error {
		for range 3 {
			value, err := store.Next(txCtx, numbering.SequenceKey{
				Scope: "tenant-1", DocType: "TRANSPORT_TASK", Period: "2026",
			})
			if err != nil {
				return err
			}
			got = append(got, value)
		}
		other, err := store.Next(txCtx, numbering.SequenceKey{
			Scope: "tenant-1", DocType: "TRANSPORT_TASK", Period: "2027",
		})
		if err != nil {
			return err
		}
		if other != 1 {
			return fmt.Errorf("a fresh period must restart at 1, got %d", other)
		}
		crossScope, err := store.Next(txCtx, numbering.SequenceKey{
			Scope: "tenant-2", DocType: "TRANSPORT_TASK", Period: "2026",
		})
		if err != nil {
			return err
		}
		if crossScope != 1 {
			return fmt.Errorf("a fresh scope must restart at 1, got %d", crossScope)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 || got[0] != 1 || got[1] != 2 || got[2] != 3 {
		t.Fatalf("expected 1,2,3 within one key, got %v", got)
	}
}

func TestNextRequiresTransaction(t *testing.T) {
	_, store := testStore(t)
	_, err := store.Next(context.Background(), numbering.SequenceKey{
		Scope: "tenant-1", DocType: "TRANSPORT_TASK",
	})
	if !errors.Is(err, bentopg.ErrTransactionRequired) {
		t.Fatalf("expected ErrTransactionRequired, got %v", err)
	}
}

func TestNextRejectsIncompleteKey(t *testing.T) {
	db, store := testStore(t)
	err := db.Transactor().WithinTransaction(context.Background(), func(txCtx context.Context) error {
		_, err := store.Next(txCtx, numbering.SequenceKey{Scope: "tenant-1"})
		return err
	})
	if !errors.Is(err, ErrKeyRequired) {
		t.Fatalf("expected ErrKeyRequired, got %v", err)
	}
}

func TestRolledBackAllocationIsReissued(t *testing.T) {
	db, store := testStore(t)
	ctx := context.Background()
	key := numbering.SequenceKey{Scope: "tenant-1", DocType: "STATEMENT", Period: "202608"}
	boom := errors.New("boom")

	next := func() (int64, error) {
		var value int64
		err := db.Transactor().WithinTransaction(ctx, func(txCtx context.Context) error {
			var err error
			value, err = store.Next(txCtx, key)
			return err
		})
		return value, err
	}

	first, err := next()
	if err != nil {
		t.Fatal(err)
	}
	if first != 1 {
		t.Fatalf("first committed allocation must be 1, got %d", first)
	}

	// 事务回滚必须释放取到的号：分配与业务写入同生共死。
	err = db.Transactor().WithinTransaction(ctx, func(txCtx context.Context) error {
		if _, err := store.Next(txCtx, key); err != nil {
			return err
		}
		return boom
	})
	if !errors.Is(err, boom) {
		t.Fatalf("expected the injected rollback, got %v", err)
	}

	second, err := next()
	if err != nil {
		t.Fatal(err)
	}
	if second != 2 {
		t.Fatalf("rolled back number must be reissued: want 2, got %d", second)
	}
}

func TestConcurrentAllocationsNeverCollide(t *testing.T) {
	db, store := testStore(t)
	key := numbering.SequenceKey{Scope: "tenant-1", DocType: "TRANSPORT_TASK", Period: "2026"}

	const workers, perWorker = 10, 10
	values := make(chan int64, workers*perWorker)
	var wg sync.WaitGroup
	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range perWorker {
				err := db.Transactor().WithinTransaction(context.Background(), func(txCtx context.Context) error {
					value, err := store.Next(txCtx, key)
					if err != nil {
						return err
					}
					values <- value
					return nil
				})
				if err != nil {
					t.Error(err)
					return
				}
			}
		}()
	}
	wg.Wait()
	close(values)

	seen := map[int64]bool{}
	count := 0
	for value := range values {
		if seen[value] {
			t.Fatalf("duplicate committed allocation %d", value)
		}
		seen[value] = true
		count++
	}
	if count != workers*perWorker {
		t.Fatalf("expected %d allocations, got %d", workers*perWorker, count)
	}
	for expected := int64(1); expected <= int64(count); expected++ {
		if !seen[expected] {
			t.Fatalf("committed sequence has a hole at %d", expected)
		}
	}
}

func TestAssignerEndToEndAgainstPostgres(t *testing.T) {
	db, store := testStore(t)
	registry, err := numbering.NewRegistry(map[string]numbering.DocTypeSpec{
		"TRANSPORT_TASK": {Prefix: "TT", Template: "TT-{yyyy}-{seq:4}"},
		"STATEMENT":      {Prefix: "STMT", Template: "STMT-{yyyy}{mm}-{seq:5}"},
	})
	if err != nil {
		t.Fatal(err)
	}
	fixed := func() time.Time { return time.Date(2026, 8, 24, 12, 0, 0, 0, time.UTC) }
	assigner, err := numbering.NewAssigner(registry, store, numbering.WithNow(fixed))
	if err != nil {
		t.Fatal(err)
	}

	assign := func(docType string) string {
		number, err := application.Transactional(
			context.Background(), db.Transactor(),
			func(txCtx context.Context) (string, error) {
				return assigner.Assign(txCtx, "tenant-1", docType)
			},
		)
		if err != nil {
			t.Fatal(err)
		}
		return number
	}

	if got := assign("TRANSPORT_TASK"); got != "TT-2026-0001" {
		t.Fatalf("got %q", got)
	}
	if got := assign("TRANSPORT_TASK"); got != "TT-2026-0002" {
		t.Fatalf("got %q", got)
	}
	if got := assign("STATEMENT"); got != "STMT-202608-00001" {
		t.Fatalf("got %q", got)
	}
}
