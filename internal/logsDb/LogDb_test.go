package logsDb_test

import (
	"testing"

	"github.com/religiosa1/webhook-receiver/internal/logsDb"
)

func TestLogDb(t *testing.T) {
	t.Run("Creates a db", func(t *testing.T) {
		_, err := logsDb.New(":memory:")
		if err != nil {
			t.Error(err)
		}
	})
}
