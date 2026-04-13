package serialization

import (
	actiondb "github.com/religiosa1/git-webhook-receiver/internal/actionDb"
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

func PipelineRecord(r actiondb.PipeLineRecord) PrettyPipelineRecord {
	config, _ := NewJSONData(r.Config)

	return PrettyPipelineRecord{
		PipeID:     r.PipeId,
		Project:    r.Project,
		DeliveryID: r.DeliveryId,
		Config:     config,
		Error:      NullString{r.Error},
		CreatedAt:  Timestamp{r.CreatedAt},
		EndedAt:    NullTS{r.EndedAt},
	}
}

func PipelineRecords(rs []actiondb.PipeLineRecord) []PrettyPipelineRecord {
	records := make([]PrettyPipelineRecord, len(rs))
	for i, r := range rs {
		records[i] = PipelineRecord(r)
	}
	return records
}
