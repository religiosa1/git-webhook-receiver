package models

type PagedDB[T any] struct {
	Items      []T
	TotalCount int
	Cursor     *string
}

type Paged[T any] struct {
	Items      []T     `json:"items"`
	TotalCount int     `json:"totalCount"`
	NextPage   *string `json:"nextPage,omitempty"`
}
