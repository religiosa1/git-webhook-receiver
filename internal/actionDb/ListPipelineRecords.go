package actiondb

import (
	"fmt"
	"strings"

	"github.com/religiosa1/git-webhook-receiver/internal/models"
	sqlfilterbuilder "github.com/religiosa1/git-webhook-receiver/internal/sqlFilterBuilder"
)

type ListPipelineRecordsQuery struct {
	Offset     int
	Limit      int
	Status     PipeStatus
	Project    string
	DeliveryID string
	Cursor     string
}

const maxPageSize int = 200

func (d ActionDB) ListPipelineRecords(search ListPipelineRecordsQuery) (models.PagedDB[PipeLineRecord], error) {
	if search.Limit <= 0 || search.Limit > maxPageSize {
		search.Limit = 20
	}
	var result models.PagedDB[PipeLineRecord]

	if search.Cursor != "" && search.Offset != 0 {
		return result, ErrCursorAndOffset
	}

	cursor, err := newCursorFromStr(search.Cursor)
	if err != nil {
		return result, err
	}

	var qb strings.Builder
	args := make([]any, 0)

	qb.WriteString(`
SELECT
	id, pipe_id, project, delivery_id, config, error, created_at, ended_at
FROM
	pipeline
`)

	fj := createListPipelineWhereQuery(search)
	if cursor != nil {
		fj.AddParamFilter("(created_at, id) < (?, ?)\n", cursor.CreatedAt, cursor.ID)
	}

	if fj.HasFilters() {
		qb.WriteString("WHERE\n")
		qb.WriteString(fj.String())
		args = append(args, fj.Args()...)
	}

	qb.WriteString("ORDER BY created_at DESC, id DESC\n")
	qb.WriteString("LIMIT ?\n")
	args = append(args, search.Limit+1) // +1 to understand if we still have next page or not

	if search.Offset != 0 {
		qb.WriteString("OFFSET ?\n")
		args = append(args, search.Offset)
	}

	var rows []PipeLineRecord
	err = d.db.Select(&rows, qb.String(), args...)
	if err != nil {
		return result, err
	}
	result.Items = rows[:min(search.Limit, len(rows))]
	result.TotalCount, err = d.CountPipelineRecords(search)
	if err != nil {
		return result, fmt.Errorf("error while getting the total count of pipeline records: %w", err)
	}

	if len(rows) > search.Limit {
		lastReturnedRow := rows[search.Limit-1]
		c := paginationCursor{
			CreatedAt: lastReturnedRow.CreatedAt,
			ID:        lastReturnedRow.ID,
		}.String()
		result.Cursor = &c
	}
	return result, nil
}

// CountPipelineRecords counts the amount of pipeline records matching provided
// search query, disregarding pagination params (offset or cursor)
func (d ActionDB) CountPipelineRecords(search ListPipelineRecordsQuery) (int, error) {
	args := make([]any, 0)
	var qb strings.Builder
	qb.WriteString(`SELECT count(*) FROM pipeline`)
	fb := createListPipelineWhereQuery(search)
	if fb.HasFilters() {
		qb.WriteString("\nWHERE\n")
		qb.WriteString(fb.String())
		args = append(args, fb.Args()...)
	}
	row := d.db.QueryRow(qb.String(), args...)
	var count int
	err := row.Scan(&count)
	if err != nil {
		return 0, err
	}
	return count, nil
}

func createListPipelineWhereQuery(search ListPipelineRecordsQuery) *sqlfilterbuilder.Builder {
	fb := sqlfilterbuilder.New()

	fb.AddEqFilter("delivery_id", search.DeliveryID)
	fb.AddEqFilter("project", search.Project)

	switch search.Status {
	case PipeStatusOk:
		fb.AddFilter("(ended_at IS NOT NULL AND (error IS NULL OR error = ''))\n")
	case PipeStatusError:
		fb.AddFilter("(ended_at IS NOT NULL AND (error IS NOT NULL AND error <> ''))\n")
	case PipeStatusPending:
		fb.AddFilter("ended_at IS NULL\n")
	}

	return fb
}
