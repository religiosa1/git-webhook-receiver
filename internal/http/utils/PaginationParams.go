package utils

import (
	"fmt"
	"net/url"
	"strconv"
)

const maxLimit = 1000

type PaginationParams struct {
	Offset int
	Limit  int
}

func ParsePagination(query url.Values) (PaginationParams, error) {
	var p PaginationParams
	var err error
	p.Offset, err = parseOptionalInt(query.Get("offset"))
	if err != nil {
		return p, fmt.Errorf("bad `offset` query param value: %w", err)
	}
	if p.Offset < 0 {
		return p, fmt.Errorf("`offset` can't be negative")
	}
	p.Limit, err = parseOptionalInt(query.Get("limit"))
	if err != nil {
		return p, fmt.Errorf("bad `limit` query param value: %w", err)
	}
	if p.Limit < 0 {
		return p, fmt.Errorf("`limit` can't be negative")
	}
	if p.Limit > maxLimit {
		return p, fmt.Errorf("`limit` cannot exceed %d", maxLimit)
	}
	return p, nil
}

func parseOptionalInt(val string) (int, error) {
	if val == "" {
		return 0, nil
	}
	return strconv.Atoi(val)
}
