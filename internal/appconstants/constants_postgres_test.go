package appconstants

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dronm/ds/v4/pgds"
	"github.com/dronm/meatshop/internal/models"
	"github.com/dronm/webapp/constants"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// Each test owns a fresh database. The configured PostgreSQL user needs CREATEDB.
func TestCashCustomerPostgresLifecycle(t *testing.T) {
	dsn := os.Getenv("MEATSHOP_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set MEATSHOP_TEST_DATABASE_URL to run isolated PostgreSQL tests")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	admin, err := pgx.Connect(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close(context.Background())
	databaseName := fmt.Sprintf("meatshop_constants_test_%d", time.Now().UnixNano())
	if _, err := admin.Exec(ctx, "CREATE DATABASE "+pgx.Identifier{databaseName}.Sanitize()); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if _, err := admin.Exec(context.Background(), "DROP DATABASE "+pgx.Identifier{databaseName}.Sanitize()+" WITH (FORCE)"); err != nil {
			t.Error(err)
		}
	}()
	testURL, err := url.Parse(dsn)
	if err != nil || testURL.Scheme != "postgres" && testURL.Scheme != "postgresql" {
		t.Fatalf("MEATSHOP_TEST_DATABASE_URL must be a PostgreSQL URL: %v", err)
	}
	testURL.Path = "/" + databaseName
	testDSN := testURL.String()
	conn, err := pgx.Connect(ctx, testDSN)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(context.Background())
	if _, err := conn.Exec(ctx, `
		CREATE TABLE public.customers (id integer PRIMARY KEY, name text NOT NULL);
		INSERT INTO public.customers VALUES (42, 'Cash customer'), (43, 'Unrelated customer');
		CREATE TYPE public.role_types AS ENUM ('admin', 'customer');
		CREATE TABLE public.permissions (code text PRIMARY KEY, description text NOT NULL);
		CREATE TABLE public.role_permissions (
			role_id public.role_types NOT NULL,
			permission_code text NOT NULL REFERENCES public.permissions(code),
			PRIMARY KEY (role_id, permission_code)
		);
	`); err != nil {
		t.Fatal(err)
	}
	migrate := func(name string) {
		t.Helper()
		sql, err := os.ReadFile(filepath.Join("..", "..", "migrations", name))
		if err != nil {
			t.Fatal(err)
		}
		if _, err := conn.Exec(ctx, string(sql)); err != nil {
			t.Fatalf("apply %s: %v", name, err)
		}
	}
	migrate("000078_webapp_constants.up.sql")
	migrate("000079_cash_customer_constant.up.sql")
	migrate("000080_constant_permissions.up.sql")
	migrate("000081_cash_customer_reference.up.sql")
	// The application registry also requires the nullable production destination.
	// Its full migration includes MAX queue changes, which this focused fixture
	// intentionally does not install.
	if _, err := conn.Exec(ctx, `INSERT INTO public.webapp_constants
		(code, name, description, value_type, value)
		VALUES ('production_max_chat_id', 'Чат MAX для производства', '', 'string', 'null'::jsonb)`); err != nil {
		t.Fatal(err)
	}
	var adminPermissions, customerPermissions int
	if err := conn.QueryRow(ctx,
		"SELECT count(*) FILTER (WHERE role_id = 'admin'), count(*) FILTER (WHERE role_id = 'customer') FROM public.role_permissions",
	).Scan(&adminPermissions, &customerPermissions); err != nil || adminPermissions != 4 || customerPermissions != 0 {
		t.Fatalf("constant permissions: admin = %d, customer = %d, err = %v", adminPermissions, customerPermissions, err)
	}
	provider, err := pgds.New(&pgds.Config{PrimaryConnStr: testDSN})
	if err != nil {
		t.Fatal(err)
	}
	defer provider.Close()
	first, err := Open(ctx, provider, testDSN)
	if err != nil {
		t.Fatalf("open constants with null seed: %v", err)
	}
	defer first.Close(context.Background())
	second, err := Open(ctx, provider, testDSN)
	if err != nil {
		t.Fatal(err)
	}
	defer second.Close(context.Background())
	if value, err := first.CashCustomer.GetValue(ctx); err != nil || value != nil {
		t.Fatalf("initial value = %#v, err = %v", value, err)
	}
	if _, err := first.Manager.SetByCode(ctx, CodeCashCustomer,
		json.RawMessage(`{"keys":{"id":99},"descr":"Missing"}`), nil,
	); !errors.Is(err, constants.ErrValidation) {
		t.Fatalf("nonexistent database customer accepted: %v", err)
	}
	ref := &models.Ref{Descr: "Cash customer"}
	ref.Keys.ID = 42
	if err := first.CashCustomer.SetValue(ctx, ref); err != nil {
		t.Fatalf("select existing database customer: %v", err)
	}
	if _, err := conn.Exec(ctx, "DELETE FROM public.customers WHERE id = 42"); err == nil {
		t.Fatal("selected cash customer could be deleted")
	} else {
		var pgErr *pgconn.PgError
		if !errors.As(err, &pgErr) || (pgErr.Code != "23001" && pgErr.Code != "23503") || pgErr.ConstraintName != "webapp_constants_cash_customer_fk" {
			t.Fatalf("customer deletion must fail with a foreign key violation: %v", err)
		}
	}
	if _, err := conn.Exec(ctx, "DELETE FROM public.customers WHERE id = 43"); err != nil {
		t.Fatalf("constant must not prevent deleting an unrelated customer: %v", err)
	}
	var versionBefore, versionAfter int64
	if err := conn.QueryRow(ctx, "SELECT version FROM public.webapp_constants WHERE code = 'cash_customer'").Scan(&versionBefore); err != nil {
		t.Fatal(err)
	}
	// The database also enforces references when writes race Go validation, or
	// when a trusted SQL tool edits the common JSON column directly.
	if _, err := conn.Exec(ctx, `UPDATE public.webapp_constants
		SET value = '{"keys":{"id":99},"descr":"Missing"}'::jsonb
		WHERE code = 'cash_customer'`); err == nil {
		t.Fatal("direct SQL accepted a nonexistent customer reference")
	} else {
		var pgErr *pgconn.PgError
		if !errors.As(err, &pgErr) || pgErr.Code != "23503" {
			t.Fatalf("missing reference must fail with a foreign key violation: %v", err)
		}
	}
	var selectedID int
	if err := conn.QueryRow(ctx, "SELECT cash_customer_id, version FROM public.webapp_constants WHERE code = 'cash_customer'").Scan(&selectedID, &versionAfter); err != nil || selectedID != 42 || versionAfter != versionBefore {
		t.Fatalf("rejected assignment changed constant: customer = %d, version = %d, err = %v", selectedID, versionAfter, err)
	}
	// LISTEN is committed by Open, so the other process should see this change.
	deadline := time.NewTimer(3 * time.Second)
	defer deadline.Stop()
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		value, err := second.CashCustomer.GetValue(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if value != nil && value.Keys.ID == 42 {
			break
		}
		select {
		case <-deadline.C:
			t.Fatal("second manager did not receive cash_customer change")
		case <-ticker.C:
		}
	}
	migrate("000079_cash_customer_constant.up.sql")
	var storedID int
	if err := conn.QueryRow(ctx,
		"SELECT (value->'keys'->>'id')::integer FROM public.webapp_constants WHERE code = 'cash_customer'",
	).Scan(&storedID); err != nil || storedID != 42 {
		t.Fatalf("reapplying seed overwrote configured value: id = %d, err = %v", storedID, err)
	}
	if err := first.CashCustomer.SetValue(ctx, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := conn.Exec(ctx, "DELETE FROM public.customers WHERE id = 42"); err != nil {
		t.Fatalf("clearing the constant should release its customer reference: %v", err)
	}
	if err := first.Close(ctx); err != nil {
		t.Fatal(err)
	}
	if err := second.Close(ctx); err != nil {
		t.Fatal(err)
	}
	migrate("000081_cash_customer_reference.down.sql")
	migrate("000080_constant_permissions.down.sql")
	if err := conn.QueryRow(ctx, "SELECT count(*) FROM public.permissions").Scan(&adminPermissions); err != nil || adminPermissions != 0 {
		t.Fatalf("permission rollback left rows: %d, err = %v", adminPermissions, err)
	}
	migrate("000079_cash_customer_constant.down.sql")
	migrate("000078_webapp_constants.down.sql")
}
