package services

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/dronm/ds/v4"
	"github.com/dronm/session"
	"github.com/dronm/meatshop/internal/apperrors"
	"github.com/dronm/meatshop/internal/models"
	"github.com/dronm/webapp"
)

var (
	// Populated with build information in main.go before the HTTP server starts.
	ProgAbout = &models.ProgAbout{};

	progAboutMx     sync.RWMutex;
	progAboutLoaded bool;
)

var externalIPHTTPClient = &http.Client{
	Timeout: 5 * time.Second,
};

type ProgAboutService struct {
	DB      ds.Provider;
	Session session.Session;
	QueryID string;
}

func NewProgAboutService(ctx webapp.ServiceContext) any {
	return &ProgAboutService{
		DB:      ctx.DB,
		Session: ctx.Session,
		QueryID: ctx.QueryID,
	};
}

func RegisterProgAboutService() {
	webapp.MustRegisterService(
		"ProgAbout",
		&ProgAboutService{},
		NewProgAboutService,
	);
}

func (s *ProgAboutService) Info(ctx context.Context) (*models.ProgAbout, error) {
	if s.Session == nil {
		return nil, apperrors.SessionRequired();
	}
	if err := s.requireDB(); err != nil {
		return nil, err;
	}

	progAboutMx.RLock();
	if progAboutLoaded {
		result := cloneProgAbout(ProgAbout);
		progAboutMx.RUnlock();

		return result, nil;
	}
	progAboutMx.RUnlock();

	progAboutMx.Lock();
	defer progAboutMx.Unlock();

	// Another request may have initialized it while this request was waiting.
	if progAboutLoaded {
		return cloneProgAbout(ProgAbout), nil;
	}

	result := cloneProgAbout(ProgAbout);
	result.DB = &models.ProgAboutDB{
		ServerType: "PostgreSQL",
	};

	poolConn, connID, err := s.DB.GetPrimary(ctx);
	if err != nil {
		return nil, fmt.Errorf("get primary database connection: %w", err);
	}
	defer s.DB.Release(poolConn, connID);

	conn := poolConn.Conn();

	var updatedAt sql.NullTime;

	if err := conn.QueryRow(
		ctx,
		`
			SELECT
				version(),
				(
					SELECT updated_at
					FROM deploy_updates
					ORDER BY updated_at DESC
					LIMIT 1
				),
				COALESCE(
					(
						SELECT
							version::text ||
							CASE
								WHEN dirty THEN ' (dirty)'
								ELSE ''
							END
						FROM schema_migrations
						LIMIT 1
					),
					'unknown'
				)
		`,
	).Scan(
		&result.DB.ServerVersion,
		&updatedAt,
		&result.DB.Migration,
	); err != nil {
		return nil, fmt.Errorf("read program information from database: %w", err);
	}

	if updatedAt.Valid {
		result.UpdatedAt = updatedAt.Time;
	}

	ip, err := externalIP(ctx);
	if err != nil {
		// External IP is informational and should not break the whole endpoint.
		slog.Warn(
			"failed to determine external IP",
			"error", err,
			"query_id", s.QueryID,
		);
	} else {
		result.Backend.IP = ip;
	}

	// Preserve the global pointer because main.go and other code may hold it.
	*ProgAbout = *result;
	progAboutLoaded = true;

	return cloneProgAbout(ProgAbout), nil;
}

func (s *ProgAboutService) requireDB() error {
	if s.DB == nil {
		return webapp.Internal("database is not initialized", nil);
	}

	return nil;
}

func cloneProgAbout(source *models.ProgAbout) *models.ProgAbout {
	if source == nil {
		return &models.ProgAbout{};
	}

	result := *source;

	if source.DB != nil {
		db := *source.DB;
		result.DB = &db;
	}

	return &result;
}

func externalIP(ctx context.Context) (string, error) {
	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodGet,
		"https://ifconfig.me/ip",
		nil,
	);
	if err != nil {
		return "", fmt.Errorf("create external IP request: %w", err);
	}

	resp, err := externalIPHTTPClient.Do(req);
	if err != nil {
		return "", fmt.Errorf("execute external IP request: %w", err);
	}
	defer resp.Body.Close();

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return "", fmt.Errorf(
			"external IP service returned status %s",
			resp.Status,
		);
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 128));
	if err != nil {
		return "", fmt.Errorf("read external IP response: %w", err);
	}

	ip := strings.TrimSpace(string(body));
	if net.ParseIP(ip) == nil {
		return "", fmt.Errorf("external IP service returned invalid IP %q", ip);
	}

	return ip, nil;
}
