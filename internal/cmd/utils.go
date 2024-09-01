package cmd

import (
	"database/sql"
	"fmt"
)

func formatLength(output sql.NullString) string {
	if !output.Valid {
		return "null"
	}

	return formatBytes(len(output.String))
}

const (
	_           = iota
	KiB float64 = 1 << (10 * iota)
	MiB
	GiB
	TiB
)

func formatBytes(bytes int) string {
	b := float64(bytes)
	switch {
	case b >= TiB:
		return fmt.Sprintf("%.2f TiB", b/TiB)
	case b >= GiB:
		return fmt.Sprintf("%.2f GiB", b/GiB)
	case b >= MiB:
		return fmt.Sprintf("%.2f MiB", b/MiB)
	case b >= KiB:
		return fmt.Sprintf("%.2f KiB", b/KiB)
	default:
		return fmt.Sprintf("%d", bytes)
	}
}
