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
	Hash       NullString `json:"hash"`
	Config     JSONData   `json:"config"`
	Error      NullString `json:"error"`
	CreatedAt  time.Time  `json:"createdAt"`
	EndedAt    NullTS     `json:"endedAt"`
}

func PipelineRecord(r actionsdb.PipeLineRecord) PrettyPipelineRecord {
	config, _ := NewJSONData(r.Config)

	return PrettyPipelineRecord{
		PipeID:     r.PipeID,
		Project:    r.Project,
		DeliveryID: r.DeliveryID,
		Hash:       NullString{r.Hash},
		Config:     config,
		Error:      NullString{r.Error},
		CreatedAt:  r.CreatedAt,
		EndedAt:    NullTS{r.EndedAt},
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
