package services

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/dronm/ds/v4"
	"github.com/dronm/modelbind"
	"github.com/dronm/session"
	"github.com/dronm/meatshop/internal/apperrors"
	"github.com/dronm/meatshop/internal/models"
	"github.com/dronm/webapp"
)

const (
	objectHistoryDefaultPageSize = 50
	objectHistoryMaxPageSize     = 500
)

var objectHistoryObjectTypePattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

type ObjectHistoryService struct {
	DB      ds.Provider
	Session session.Session
	QueryID string
}

type objectHistoryRawChange struct {
	Column         string  `json:"col"`
	Alias          string  `json:"alias"`
	Old            any     `json:"old"`
	New            any     `json:"new"`
	OldDescription *string `json:"old_descr"`
	NewDescription *string `json:"new_descr"`
}

func NewObjectHistoryService(ctx webapp.ServiceContext) any {
	return &ObjectHistoryService{
		DB:      ctx.DB,
		Session: ctx.Session,
		QueryID: ctx.QueryID,
	}
}

func RegisterObjectHistoryService() {
	webapp.MustRegisterService(
		"ObjectHistory",
		&ObjectHistoryService{},
		NewObjectHistoryService,
	)
}

func (s *ObjectHistoryService) List(
	ctx context.Context,
	input models.ObjectHistoryInput,
) (models.ObjectHistoryResponse, error) {
	if s.Session == nil {
		return models.ObjectHistoryResponse{}, apperrors.SessionRequired()
	}
	if s.DB == nil {
		return models.ObjectHistoryResponse{}, webapp.Internal("database is not initialized", nil)
	}

	query, params, err := validateObjectHistoryInput(input)
	if err != nil {
		return models.ObjectHistoryResponse{}, err
	}

	poolConn, connID, err := s.DB.GetPrimary(ctx)
	if err != nil {
		return models.ObjectHistoryResponse{}, fmt.Errorf("get primary connection for object history: %w", err)
	}
	defer s.DB.Release(poolConn, connID)

	rows, err := poolConn.Conn().Query(ctx, `
		SELECT
			id,
			COALESCE(changed_at, to_timestamp(0)) AS changed_at,
			TRIM(operation) AS operation,
			changed_by,
			COALESCE(changes, '[]'::jsonb) AS changes,
			COUNT(*) OVER()::bigint AS total_count
		FROM public.audit_log
		WHERE table_name = $1
			AND record_id = $2
		ORDER BY changed_at DESC NULLS LAST, id DESC
		LIMIT $3 OFFSET $4
	`,
		query.ObjectType,
		query.ObjectID,
		int(params.Count),
		int(params.From),
	)
	if err != nil {
		return models.ObjectHistoryResponse{}, fmt.Errorf("select object history: %w", err)
	}
	defer rows.Close()

	result := models.ObjectHistoryResponse{
		Rows: make([]*models.ObjectHistoryRow, 0),
	}
	for rows.Next() {
		row := &models.ObjectHistoryRow{}
		var rawChanges []byte
		if err := rows.Scan(
			&row.ID,
			&row.ChangedAt,
			&row.Operation,
			&row.ChangedBy,
			&rawChanges,
			&result.Total,
		); err != nil {
			return models.ObjectHistoryResponse{}, fmt.Errorf("scan object history row: %w", err)
		}

		row.Changes, err = parseObjectHistoryChanges(rawChanges)
		if err != nil {
			return models.ObjectHistoryResponse{}, fmt.Errorf("parse object history row %d: %w", row.ID, err)
		}
		result.Rows = append(result.Rows, row)
	}
	if err := rows.Err(); err != nil {
		return models.ObjectHistoryResponse{}, fmt.Errorf("iterate object history: %w", err)
	}

	return result, nil
}

func validateObjectHistoryInput(
	input models.ObjectHistoryInput,
) (*models.ObjectHistoryQuery, modelbind.CollectionParams, error) {
	if input.Query == nil {
		return nil, modelbind.CollectionParams{}, webapp.BadRequest("object history query is required", nil)
	}

	query := *input.Query
	query.ObjectType = strings.TrimSpace(query.ObjectType)
	query.ObjectID = strings.TrimSpace(query.ObjectID)

	if query.ObjectType == "" {
		return nil, modelbind.CollectionParams{}, webapp.BadRequest("object_type is required", nil)
	}
	if len(query.ObjectType) > 63 || !objectHistoryObjectTypePattern.MatchString(query.ObjectType) {
		return nil, modelbind.CollectionParams{}, webapp.BadRequest(
			"object_type should be a valid unqualified PostgreSQL table name",
			nil,
		)
	}
	if query.ObjectID == "" {
		return nil, modelbind.CollectionParams{}, webapp.BadRequest("object_id is required", nil)
	}
	if len(query.ObjectID) > 1024 {
		return nil, modelbind.CollectionParams{}, webapp.BadRequest("object_id is too long", nil)
	}

	params := input.Params
	if len(params.Filter) > 0 || len(params.Sorter) > 0 {
		return nil, modelbind.CollectionParams{}, webapp.BadRequest(
			"generic filters and sorting are not supported for object history",
			nil,
		)
	}
	if params.From < 0 {
		return nil, modelbind.CollectionParams{}, webapp.BadRequest("from should not be negative", nil)
	}
	if params.Count < 0 {
		return nil, modelbind.CollectionParams{}, webapp.BadRequest("count should not be negative", nil)
	}
	if params.Count == 0 {
		params.Count = objectHistoryDefaultPageSize
	}
	if params.Count > objectHistoryMaxPageSize {
		params.Count = objectHistoryMaxPageSize
	}

	return &query, params, nil
}

func parseObjectHistoryChanges(raw []byte) ([]*models.ObjectHistoryChange, error) {
	rawChanges := make([]objectHistoryRawChange, 0)
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if err := decoder.Decode(&rawChanges); err != nil {
		return nil, err
	}

	changes := make([]*models.ObjectHistoryChange, 0, len(rawChanges))
	for _, rawChange := range rawChanges {
		column := strings.TrimSpace(rawChange.Column)
		if column == "" || isSensitiveAuditColumn(column) {
			continue
		}

		field := strings.TrimSpace(rawChange.Alias)
		if field == "" {
			field = column
		}

		changes = append(changes, &models.ObjectHistoryChange{
			Column:         column,
			Field:          field,
			Old:            rawChange.Old,
			New:            rawChange.New,
			OldDescription: normalizedAuditDescription(rawChange.OldDescription),
			NewDescription: normalizedAuditDescription(rawChange.NewDescription),
		})
	}

	return changes, nil
}

func normalizedAuditDescription(value *string) *string {
	if value == nil {
		return nil
	}
	normalized := strings.TrimSpace(*value)
	if normalized == "" {
		return nil
	}
	return &normalized
}

func isSensitiveAuditColumn(column string) bool {
	switch strings.ToLower(strings.TrimSpace(column)) {
	case "pwd", "passwd", "password", "password_hash", "password_digest", "secret", "token", "api_key", "private_key":
		return true
	default:
		return false
	}
}
