package maxbot

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestProductionQueuePostgresDestinationsAndOrdering(t *testing.T) {
	dsn := os.Getenv("MEATSHOP_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set MEATSHOP_TEST_DATABASE_URL to run PostgreSQL queue tests")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	admin, err := pgx.Connect(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close(context.Background())
	database := fmt.Sprintf("meatshop_production_queue_%d", time.Now().UnixNano())
	if _, err := admin.Exec(ctx, "CREATE DATABASE "+pgx.Identifier{database}.Sanitize()); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if _, err := admin.Exec(context.Background(), "DROP DATABASE "+pgx.Identifier{database}.Sanitize()+" WITH (FORCE)"); err != nil {
			t.Error(err)
		}
	}()
	testURL, err := url.Parse(dsn)
	if err != nil {
		t.Fatal(err)
	}
	testURL.Path = "/" + database
	pool, err := pgxpool.New(ctx, testURL.String())
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if _, err := pool.Exec(ctx, `
		CREATE TYPE public.role_types AS ENUM ('admin', 'customer');
		CREATE TABLE public.permissions(code text PRIMARY KEY, description text NOT NULL);
		CREATE TABLE public.role_permissions(role_id public.role_types, permission_code text REFERENCES public.permissions(code), PRIMARY KEY(role_id,permission_code));
		CREATE TABLE public.max_users(id integer PRIMARY KEY, max_user_id bigint UNIQUE, username text);
		CREATE FUNCTION public.max_users_ref(public.max_users) RETURNS json LANGUAGE sql AS $$
			SELECT CASE WHEN $1.id IS NULL THEN NULL ELSE json_build_object('keys',json_build_object('id',$1.id),'descr',$1.username) END
		$$;
		CREATE TABLE public.webapp_constants(code text PRIMARY KEY,name text,description text,value_type text,value jsonb);
		INSERT INTO public.max_users VALUES(1,101,'legacy user');
	`); err != nil {
		t.Fatal(err)
	}
	for _, migration := range []string{"000033_max_bot_messages.up.sql", "000087_production_max_chat.up.sql"} {
		body, err := os.ReadFile(filepath.Join("..", "..", "migrations", migration))
		if err != nil {
			t.Fatal(err)
		}
		if _, err := pool.Exec(ctx, string(body)); err != nil {
			t.Fatalf("apply %s: %v", migration, err)
		}
	}
	store := NewStore(pool)
	var legacyID, firstID, secondID int64
	if err := pool.QueryRow(ctx, `INSERT INTO public.max_out_messages(max_user_id,message) VALUES(101,'{"text":"legacy"}') RETURNING id`).Scan(&legacyID); err != nil {
		t.Fatal(err)
	}
	const chatID int64 = -9007199254740993
	if err := pool.QueryRow(ctx, `INSERT INTO public.max_out_messages(max_chat_id,message,sequence_key,sequence_num,delivery_key,next_attempt_at)
		VALUES($1,'{"text":"details"}','production:1',1,'production:1:1',now()+interval '1 hour') RETURNING id`, chatID).Scan(&firstID); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `INSERT INTO public.max_out_messages(max_chat_id,message,sequence_key,sequence_num,delivery_key)
		VALUES($1,'{"text":"totals"}','production:1',2,'production:1:2') RETURNING id`, chatID).Scan(&secondID); err != nil {
		t.Fatal(err)
	}
	claimed, err := store.ClaimNext(ctx)
	if err != nil || claimed == nil || claimed.ID != legacyID || claimed.MaxUserID == nil || *claimed.MaxUserID != 101 || claimed.MaxChatID != nil {
		t.Fatalf("legacy destination: message=%#v, err=%v", claimed, err)
	}
	if err := store.MarkSent(ctx, legacyID, "legacy-remote"); err != nil {
		t.Fatal(err)
	}
	assertBlocked := func(reason string) {
		t.Helper()
		message, err := store.ClaimNext(ctx)
		if err != nil || message != nil {
			t.Fatalf("%s: message=%#v, err=%v", reason, message, err)
		}
	}
	assertBlocked("deferred details must block totals")
	if _, err := pool.Exec(ctx, `UPDATE public.max_out_messages SET next_attempt_at=now() WHERE id=$1`, firstID); err != nil {
		t.Fatal(err)
	}
	claimed, err = store.ClaimNext(ctx)
	if err != nil || claimed == nil || claimed.ID != firstID || claimed.MaxChatID == nil || *claimed.MaxChatID != chatID || claimed.MaxUserID != nil {
		t.Fatalf("chat int64 destination: message=%#v, err=%v", claimed, err)
	}
	if err := store.MarkFailed(ctx, firstID, errors.New("temporary fixture failure")); err != nil {
		t.Fatal(err)
	}
	assertBlocked("failed details must block totals")
	if _, err := pool.Exec(ctx, `UPDATE public.max_out_messages SET status='superseded' WHERE id=$1`, firstID); err != nil {
		t.Fatal(err)
	}
	claimed, err = store.ClaimNext(ctx)
	if err != nil || claimed == nil || claimed.ID != secondID {
		t.Fatalf("superseded details should unblock totals: message=%#v, err=%v", claimed, err)
	}
	if err := store.MarkSent(ctx, secondID, "chat-remote"); err != nil {
		t.Fatal(err)
	}
	if err := store.MarkRetry(ctx, firstID, time.Second, errors.New("late sender result")); err != nil {
		t.Fatal(err)
	}
	var state, remoteID string
	if err := pool.QueryRow(ctx, `SELECT status FROM public.max_out_messages WHERE id=$1`, firstID).Scan(&state); err != nil || state != "superseded" {
		t.Fatalf("late sender result resurrected superseded message: state=%s, err=%v", state, err)
	}
	if err := pool.QueryRow(ctx, `SELECT remote_message_id FROM public.max_out_messages WHERE id=$1`, secondID).Scan(&remoteID); err != nil || remoteID != "chat-remote" {
		t.Fatalf("remote message identity: id=%s, err=%v", remoteID, err)
	}
	for _, invalid := range []struct {
		name, sql, code string
	}{
		{"no destination", `INSERT INTO public.max_out_messages(message) VALUES('{}')`, "23514"},
		{"both destinations", `INSERT INTO public.max_out_messages(max_user_id,max_chat_id,message) VALUES(101,-123,'{}')`, "23514"},
		{"zero chat", `INSERT INTO public.max_out_messages(max_chat_id,message) VALUES(0,'{}')`, "23514"},
		{"duplicate delivery", `INSERT INTO public.max_out_messages(max_chat_id,message,delivery_key) VALUES(-123,'{}','production:1:1')`, "23505"},
	} {
		_, err := pool.Exec(ctx, invalid.sql)
		var pgErr *pgconn.PgError
		if !errors.As(err, &pgErr) || pgErr.Code != invalid.code {
			t.Fatalf("%s should fail with %s: %v", invalid.name, invalid.code, err)
		}
	}
}
