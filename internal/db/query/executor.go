package query

import (
	"context"
	"encoding/json"
	"io"

	"github.com/go-errors/errors"
	"github.com/jackc/pgconn"
	"github.com/jackc/pgx/v4"
	"github.com/volcengine/byted-supabase-cli/internal/utils"
)

type QueryResult struct {
	Columns []string
	Rows    [][]interface{}
}

type SQLExecutor interface {
	Query(context.Context, string) (QueryResult, error)
}

func RunExecutor(ctx context.Context, executor SQLExecutor, sql, format string, agentMode bool, w io.Writer) error {
	result, err := executor.Query(ctx, sql)
	if err != nil {
		return err
	}
	return formatOutput(w, format, agentMode, result.Columns, result.Rows, nil)
}

type PostgresExecutor struct {
	Config  pgconn.Config
	Options []func(*pgx.ConnConfig)
}

func (e PostgresExecutor) Query(ctx context.Context, sql string) (QueryResult, error) {
	conn, err := utils.ConnectByConfig(ctx, e.Config, e.Options...)
	if err != nil {
		return QueryResult{}, err
	}
	defer conn.Close(ctx)
	rows, err := conn.Query(ctx, sql)
	if err != nil {
		return QueryResult{}, errors.Errorf("failed to execute query: %w", err)
	}
	defer rows.Close()
	fields := rows.FieldDescriptions()
	result := QueryResult{Columns: make([]string, len(fields))}
	for i, field := range fields {
		result.Columns[i] = string(field.Name)
	}
	for rows.Next() {
		values := make([]interface{}, len(result.Columns))
		targets := make([]interface{}, len(values))
		for i := range values {
			targets[i] = &values[i]
		}
		if err := rows.Scan(targets...); err != nil {
			return QueryResult{}, err
		}
		result.Rows = append(result.Rows, values)
	}
	return result, rows.Err()
}

type SupabaseDataPlaneExecutor struct {
	SupabaseURL    string
	ServiceRoleKey string
}

func (e SupabaseDataPlaneExecutor) Query(ctx context.Context, sql string) (QueryResult, error) {
	body, err := ExecuteSupabaseHTTP(ctx, SupabaseHTTPParams{
		SupabaseURL: e.SupabaseURL, ServiceRoleKey: e.ServiceRoleKey, SQL: sql,
	})
	if err != nil {
		return QueryResult{}, err
	}
	var rows []map[string]interface{}
	if err := json.Unmarshal(body, &rows); err != nil {
		return QueryResult{}, nil
	}
	result := QueryResult{}
	if len(rows) == 0 {
		return result, nil
	}
	for column := range rows[0] {
		result.Columns = append(result.Columns, column)
	}
	for _, row := range rows {
		values := make([]interface{}, len(result.Columns))
		for i, column := range result.Columns {
			values[i] = row[column]
		}
		result.Rows = append(result.Rows, values)
	}
	return result, nil
}
