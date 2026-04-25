package actiondb

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

func encodeCursor(c paginationCursor) string {
	return fmt.Sprintf("%d_%d", c.CreatedAt, c.ID)
}

func decodeCursor(s string) (paginationCursor, error) {
	i := strings.IndexByte(s, '_')
	if i < 0 || i == 0 || i == len(s)-1 {
		return paginationCursor{}, ErrBadCursor
	}
	createdAt, err := strconv.ParseInt(s[:i], 10, 64)
	if err != nil {
		return paginationCursor{}, ErrBadCursor
	}
	id, err := strconv.ParseInt(s[i+1:], 10, 64)
	if err != nil {
		return paginationCursor{}, ErrBadCursor
	}
	return paginationCursor{CreatedAt: createdAt, ID: id}, nil
}
