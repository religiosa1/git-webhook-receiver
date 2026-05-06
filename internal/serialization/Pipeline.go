package serialization

import (
	"github.com/religiosa1/git-webhook-receiver/internal/actionsdb"
	"github.com/religiosa1/git-webhook-receiver/internal/models"
)

type PrettyPipelineRecord struct {
	PipeID     string     `json:"pipeId"`
	Project    string     `json:"project"`
	DeliveryID string     `json:"deliveryId"`
	Config     JSONData   `json:"config"`
	Error      NullString `json:"error"`
	CreatedAt  Timestamp  `json:"createdAt"`
	EndedAt    NullTS     `json:"endedAt"`
}

func PipelineRecord(r actionsdb.PipeLineRecord) PrettyPipelineRecord {
	config, _ := NewJSONData(r.Config)

	return PrettyPipelineRecord{
		PipeID:     r.PipeID,
		Project:    r.Project,
		DeliveryID: r.DeliveryID,
		Config:     config,
		Error:      NullString{r.Error},
		CreatedAt:  Timestamp{r.CreatedAt},
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
