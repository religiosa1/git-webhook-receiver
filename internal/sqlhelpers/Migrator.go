package sqlhelpers

import (
	"database/sql"
	"errors"
	"fmt"

	"github.com/jmoiron/sqlx"
)

// Migrator is a sqlite migrator, which uses PRAGMA user_version to track the migrations applied;
// https://sqlite.org/pragma.html#pragma_user_version
// Migrations are applied forward-only, you can't rollback a migration, once applied.
type Migrator struct {
	db *sqlx.DB
}

func NewMigrator(db *sqlx.DB) Migrator {
	return Migrator{db: db}
}

// Migrate migrates the DB schema if necessary based on the migrations provided
// in args and bumps the user_version of DB
func (m Migrator) Migrate(migrations []string) (err error) {
	userVersion, err := m.getUserVersion()
	if err != nil {
		return err
	}
	targetUserVersion := len(migrations)

	if userVersion == targetUserVersion {
		return nil
	}
	if userVersion > targetUserVersion {
		return fmt.Errorf(
			"current user version in DB (%d) is higher than the expected version: %d",
			userVersion,
			targetUserVersion,
		)
	}

	tx, err := m.db.Begin()
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			err = errors.Join(err, tx.Rollback())
			return
		}
		err = tx.Commit()
	}()

	for i := userVersion; i < targetUserVersion; i++ {
		_, err = tx.Exec(migrations[i])
		if err != nil {
			return err
		}
	}
	err = setUserVersion(tx, targetUserVersion)
	return err
}

func (m Migrator) getUserVersion() (int, error) {
	var userVersion int
	err := m.db.Get(&userVersion, "PRAGMA user_version;")
	if err != nil {
		return -1, err
	}
	return userVersion, nil
}

func setUserVersion(tx *sql.Tx, value int) error {
	// sqlite doesn't support parameterized pragmas, hence sprintf.
	// Safe here anyway, as it's an int.
	_, err := tx.Exec(fmt.Sprintf("PRAGMA user_version = %d;", value))
	return err
}
