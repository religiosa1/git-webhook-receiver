package actiondb

import (
	"fmt"
	"strings"

	"github.com/religiosa1/git-webhook-receiver/internal/models"
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

	var cursor *paginationCursor
	if search.Cursor != "" {
		c, err := decodeCursor(search.Cursor)
		if err != nil {
			return result, err
		}
		cursor = &c
	}

	var qb strings.Builder
	args := make([]any, 0)

	// TODO: make sense of indexes both in Actions and Logs databases
	qb.WriteString(`
SELECT
	id, pipe_id, project, delivery_id, config, error, created_at, ended_at
FROM
	pipeline
`)

	fj := createListPipelineWhereQuery(search)
	if cursor != nil {
		fj.AddParamFilter(
			"(created_at < ? OR (created_at = ? AND id < ?))\n",
			cursor.CreatedAt, cursor.CreatedAt, cursor.ID,
		)
	}

	if fj.HasFilters {
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
	err := d.db.Select(&rows, qb.String(), args...)
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
		c := encodeCursor(paginationCursor{
			CreatedAt: lastReturnedRow.CreatedAt,
			ID:        lastReturnedRow.ID,
		})
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
	fj := createListPipelineWhereQuery(search)
	if fj.HasFilters {
		qb.WriteString("\nWHERE\n")
		qb.WriteString(fj.String())
		args = append(args, fj.Args()...)
	}
	row := d.db.QueryRow(qb.String(), args...)
	var count int
	err := row.Scan(&count)
	if err != nil {
		return 0, err
	}
	return count, nil
}

func createListPipelineWhereQuery(search ListPipelineRecordsQuery) filterJoiner {
	fj := filterJoiner{}

	// TODO: do we really need LIKE filter here instead of just '='?
	fj.AddLikeFilter(search.DeliveryID, "delivery_id")
	fj.AddLikeFilter(search.Project, "project")

	switch search.Status {
	case PipeStatusOk:
		fj.AddFilter("(ended_at IS NOT NULL AND (error IS NULL OR error = ''))\n")
	case PipeStatusError:
		fj.AddFilter("(ended_at IS NOT NULL AND (error IS NOT NULL AND error <> ''))\n")
	case PipeStatusPending:
		fj.AddFilter("ended_at IS NULL\n")
	}

	return fj
}
