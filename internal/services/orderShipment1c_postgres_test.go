package services

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/dronm/ds/v4"
	integration "github.com/dronm/meatshop/internal/integration1c"
	"github.com/dronm/meatshop/internal/models"
	"github.com/jackc/pgx/v5/pgxpool"
)

func shipmentReadinessPostgres(t *testing.T) (*pgxpool.Pool, ds.Provider) {
	t.Helper()
	pool, provider := customerMessagePostgres(t)
	ctx := context.Background()
	if _, err := pool.Exec(ctx, `
		ALTER TABLE public.customers ADD COLUMN ref_1c jsonb;
		UPDATE public.customers SET ref_1c = '{"id":"customer-1c"}' WHERE id = 1;
		CREATE TABLE public.orders (
			id integer PRIMARY KEY, version bigint NOT NULL, customer_id integer,
			ref_1c jsonb, cash_payed boolean NOT NULL DEFAULT false,
			customer_name text, customer_phone text
		);
		CREATE TABLE public.products (id integer PRIMARY KEY, ref_1c jsonb);
		CREATE TABLE public.order_items (
			id integer PRIMARY KEY, order_id integer, line_num integer, product_id integer,
			quant_required numeric, quant numeric
		);
		INSERT INTO public.orders VALUES (101, 2, 1, '{"id":"invoice-1c"}', true, 'Иван', '+79121234567');
		INSERT INTO public.products VALUES (10, '{"id":"product-1c"}');
		INSERT INTO public.order_items VALUES (201, 101, 1, 10, 2, 2);
	`); err != nil {
		t.Fatal(err)
	}
	migration, err := os.ReadFile("../../migrations/000026_integration_1c_queue.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, string(migration)); err != nil {
		t.Fatal(err)
	}
	return pool, provider
}

func insertShipmentInvoiceSync(t *testing.T, pool *pgxpool.Pool, sync shipmentInvoiceSync) int64 {
	t.Helper()
	ctx := context.Background()
	var jobID, attemptID int64
	if err := pool.QueryRow(ctx, `INSERT INTO integration_1c.jobs (command, params, metadata, status)
		VALUES ($1, $2, $3, $4) RETURNING id`, *sync.command, sync.params, sync.metadata, *sync.status).Scan(&jobID); err != nil {
		t.Fatal(err)
	}
	if sync.outcome == nil {
		return jobID
	}
	if err := pool.QueryRow(ctx, `INSERT INTO integration_1c.attempts (job_id, attempt_no, worker_id)
		VALUES ($1, 1, 'test-worker') RETURNING id`, jobID).Scan(&attemptID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO integration_1c.results
		(job_id, attempt_id, outcome, http_status, body, consumed_at)
		VALUES ($1, $2, $3, $4, $5, CASE WHEN $6 THEN now() END)`,
		jobID, attemptID, *sync.outcome, sync.httpStatus, sync.body, sync.consumed); err != nil {
		t.Fatal(err)
	}
	return jobID
}

func TestShipmentPostgresReadinessGatesEnqueue(t *testing.T) {
	pool, provider := shipmentReadinessPostgres(t)
	ctx := context.Background()
	service := &OrderService{DB: provider, Session: customerMessageTestSession{user: models.UserLogin{ID: 101, RoleID: models.RoleIDAdmin}}}
	for _, test := range []struct {
		name      string
		batch     bool
		change    func(*shipmentInvoiceSync)
		wantReady bool
	}{
		{"legacy linked invoice without jobs", false, nil, true},
		{"single current success", false, func(*shipmentInvoiceSync) {}, true},
		{"batch current success", true, func(*shipmentInvoiceSync) {}, true},
		{"queued correction", false, func(sync *shipmentInvoiceSync) { status := "queued"; sync.status = &status }, false},
		{"processing correction", false, func(sync *shipmentInvoiceSync) { status := "processing"; sync.status = &status }, false},
		{"result not consumed", false, func(sync *shipmentInvoiceSync) { sync.consumed = false }, false},
		{"business failure", false, func(sync *shipmentInvoiceSync) { sync.body = []byte(`{"success":false,"error":"failed"}`) }, false},
		{"older snapshot", false, func(sync *shipmentInvoiceSync) {
			var order integration.CreateOrderParams
			if err := json.Unmarshal(sync.params, &order); err != nil {
				t.Fatal(err)
			}
			order.OrderVersion--
			var err error
			sync.params, err = json.Marshal(order)
			if err != nil {
				t.Fatal(err)
			}
		}, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			if _, err := pool.Exec(ctx, `TRUNCATE integration_1c.jobs CASCADE`); err != nil {
				t.Fatal(err)
			}
			if test.change != nil {
				_, sync := shipmentSyncFixture(t, test.batch)
				test.change(&sync)
				insertShipmentInvoiceSync(t, pool, sync)
			}
			response, err := service.CreateShipments1C(ctx, models.OrderIDsRequest{OrderIDs: []int{101}})
			if (err == nil) != test.wantReady {
				t.Fatalf("ready=%t, response=%#v, err=%v", test.wantReady, response, err)
			}
			var count int
			if err := pool.QueryRow(ctx, `SELECT count(*) FROM integration_1c.jobs WHERE command = 'create_shipments'`).Scan(&count); err != nil {
				t.Fatal(err)
			}
			if !test.wantReady {
				if count != 0 {
					t.Fatal("blocked invoice still queued a shipment")
				}
				return
			}
			var paramsJSON []byte
			if err := pool.QueryRow(ctx, `SELECT params FROM integration_1c.jobs WHERE id = $1`, response.JobID).Scan(&paramsJSON); err != nil {
				t.Fatal(err)
			}
			var params integration.CreateShipmentsParams
			if err := json.Unmarshal(paramsJSON, &params); err != nil {
				t.Fatal(err)
			}
			if test.change == nil {
				if len(params.OrderVersions) != 0 {
					t.Fatal("legacy invoice incorrectly required a known 1C version")
				}
			} else if len(params.OrderVersions) != 1 || params.OrderVersions[0].OrderVersion != 2 {
				t.Fatalf("accepted invoice version was not pinned: %s", paramsJSON)
			}
		})
	}
}

func TestShipmentPostgresReadinessWaitsForConsumerCommit(t *testing.T) {
	pool, provider := shipmentReadinessPostgres(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, sync := shipmentSyncFixture(t, false)
	sync.consumed = false
	jobID := insertShipmentInvoiceSync(t, pool, sync)
	consumer, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer consumer.Rollback(context.Background())
	if _, err := consumer.Exec(ctx, `SELECT id FROM public.orders WHERE id = 101 FOR UPDATE`); err != nil {
		t.Fatal(err)
	}
	service := &OrderService{DB: provider, Session: customerMessageTestSession{user: models.UserLogin{ID: 101, RoleID: models.RoleIDAdmin}}}
	done := make(chan error, 1)
	go func() {
		_, err := service.CreateShipments1C(ctx, models.OrderIDsRequest{OrderIDs: []int{101}})
		done <- err
	}()
	select {
	case err := <-done:
		t.Fatalf("shipment passed an uncommitted result consumer: %v", err)
	case <-time.After(100 * time.Millisecond):
	}
	if _, err := consumer.Exec(ctx, `UPDATE integration_1c.results SET consumed_at = now() WHERE job_id = $1`, jobID); err != nil {
		t.Fatal(err)
	}
	if err := consumer.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	if err := <-done; err != nil {
		t.Fatalf("committed current synchronization should allow shipment: %v", err)
	}
}
