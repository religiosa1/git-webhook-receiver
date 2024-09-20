package serialization

import actiondb "github.com/religiosa1/git-webhook-receiver/internal/actionDb"

type PrettyPipelineRecord struct {
	PipeId     string     `json:"pipeId"`
	Project    string     `json:"project"`
	DeliveryId string     `json:"deliveryId"`
	Config     JsonData   `json:"config"`
	Error      NullString `json:"error"`
	CreatedAt  Timestamp  `json:"createdAt"`
	EndedAt    NullTs     `json:"endedAt"`
}

func PipelineRecord(r actiondb.PipeLineRecord) PrettyPipelineRecord {
	config, _ := NewJsonData(r.Config)

	return PrettyPipelineRecord{
		PipeId:     r.PipeId,
		Project:    r.Project,
		DeliveryId: r.DeliveryId,
		Config:     config,
		Error:      NullString{r.Error},
		CreatedAt:  Timestamp{r.CreatedAt},
		EndedAt:    NullTs{r.EndedAt},
	}
}

func PipelineRecords(rs []actiondb.PipeLineRecord) []PrettyPipelineRecord {
	records := make([]PrettyPipelineRecord, len(rs))
	for i, r := range rs {
		records[i] = PipelineRecord(r)
	}
	return records
}
