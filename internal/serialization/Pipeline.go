package serialization

import (
	"time"

	"github.com/religiosa1/git-webhook-receiver/internal/actionsdb"
	"github.com/religiosa1/git-webhook-receiver/internal/models"
)

type PrettyPipelineRecord struct {
	PipeID     string     `json:"pipeId"`
	Project    string     `json:"project"`
	DeliveryID string     `json:"deliveryId"`
	Hash       *string    `json:"hash"`
	Config     JSONData   `json:"config"`
	Error      *string    `json:"error"`
	CreatedAt  time.Time  `json:"createdAt"`
	EndedAt    *time.Time `json:"endedAt"`
}

func PipelineRecord(r actionsdb.PipeLineRecord) PrettyPipelineRecord {
	config, _ := NewJSONData(r.Config)

	var hash *string
	if r.Hash != "" {
		hash = &r.Hash
	}
	var errStr *string
	if r.Error != nil {
		s := r.Error.Error()
		errStr = &s
	}

	return PrettyPipelineRecord{
		PipeID:     r.PipeID,
		Project:    r.Project,
		DeliveryID: r.DeliveryID,
		Hash:       hash,
		Config:     config,
		Error:      errStr,
		CreatedAt:  r.CreatedAt,
		EndedAt:    r.EndedAt,
	}
}

func PipelineRecords(rs []actionsdb.PipeLineRecord) []PrettyPipelineRecord {
	records := make([]PrettyPipelineRecord, len(rs))
	for i, r := range rs {
		records[i] = PipelineRecord(r)
	}
	return records
}

func PipelinePage(pagedPipeLineRecords models.PagedDB[actionsdb.PipeLineRecord]) models.Paged[PrettyPipelineRecord] {
	var result models.Paged[PrettyPipelineRecord]
	result.TotalCount = pagedPipeLineRecords.TotalCount
	result.Items = PipelineRecords(pagedPipeLineRecords.Items)
	return result
}
