package main

import (
	"fmt"

	"github.com/go-pg/pg/v10"
	migrate "github.com/lawzava/go-pg-migrate"
)

var m3 = &migrate.Migration{
	Name:   "Add Address For Users",
	Number: 3,
	Forwards: func(tx *pg.Tx) error {
		_, err := tx.Exec("ALTER TABLE users ADD COLUMN address TEXT")
		if err != nil {
			return fmt.Errorf("failed to alter users table to add address: %w", err)
		}

		return nil
	},
	Backwards: func(tx *pg.Tx) error {
		_, err := tx.Exec("ALTER TABLE users DROP COLUMN address")
		if err != nil {
			return fmt.Errorf("failed to drop address column for users table: %w", err)
		}

		return nil
	},
}
