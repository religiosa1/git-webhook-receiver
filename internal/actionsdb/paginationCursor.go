package actionsdb

import (
	"fmt"
	"strconv"
	"strings"
)

// paginationCursor is a db cursor used in list pipeline records
type paginationCursor struct {
	CreatedAt int64
	ID        int64
}

func (c paginationCursor) String() string {
	return fmt.Sprintf("%d_%d", c.CreatedAt, c.ID)
}

func newCursorFromStr(s string) (*paginationCursor, error) {
	if s == "" {
		return nil, nil
	}
	i := strings.IndexByte(s, '_')
	if i < 0 || i == 0 || i == len(s)-1 {
		return nil, ErrBadCursor
	}
	createdAt, err := strconv.ParseInt(s[:i], 10, 64)
	if err != nil {
		return nil, ErrBadCursor
	}
	id, err := strconv.ParseInt(s[i+1:], 10, 64)
	if err != nil {
		return nil, ErrBadCursor
	}
	return &paginationCursor{CreatedAt: createdAt, ID: id}, nil
}
