package migrate

import (
	"errors"
	"fmt"
	"sort"

	"github.com/go-pg/pg/v10"
	"github.com/rs/zerolog/log"
)

var errNoMigrationVersion = errors.New("migration version not found")

type Options struct {
	VersionNumberToApply          uint
	PrintVersionAndExit           bool
	ForceVersionWithoutMigrations bool
	RefreshSchema                 bool
}

type Migrate struct {
	migrations []*migration
	repo       *repository

	opt Options
}

func New(db *pg.DB, migrations []*Migration, opt Options) *Migrate {
	return &Migrate{
		migrations: mapMigrations(migrations),
		repo:       newRepo(db),
		opt:        opt,
	}
}

func (m Migrate) Migrate() error {
	if m.opt.RefreshSchema {
		if err := m.refreshDatabase(); err != nil {
			return fmt.Errorf("refreshing database: %w", err)
		}
	} else {
		err := m.repo.IncreaseErrorVerbosity()
		if err != nil {
			return fmt.Errorf("failed to increase DB verbosity: %w", err)
		}

		err = m.repo.EnsureMigrationTable()
		if err != nil {
			return fmt.Errorf("failed to automatically migrate migrations table: %w", err)
		}
	}

	if m.opt.ForceVersionWithoutMigrations {
		for _, migration := range m.migrations {
			if migration.Number != m.opt.VersionNumberToApply {
				continue
			}

			if err := m.repo.RemoveMigrationsAfter(migration.Number); err != nil {
				return fmt.Errorf("failed to remove migrations: %w", err)
			}

			if err := m.repo.InsertMigration(migration); err != nil {
				return fmt.Errorf("failed insert migration: %w", err)
			}

			return nil
		}

		return errNoMigrationVersion
	}

	lastAppliedMigrationNumber, err := m.repo.GetLatestMigrationNumber()
	if err != nil {
		return fmt.Errorf("failed to get the number of the latest migration: %w", err)
	}

	if m.opt.PrintVersionAndExit {
		log.Info().Msgf("currently applied version: %d", lastAppliedMigrationNumber)

		return nil
	}

	if err := m.applyMigrations(lastAppliedMigrationNumber); err != nil {
		return fmt.Errorf("failed to apply migrations: %w", err)
	}

	return nil
}

func (m Migrate) refreshDatabase() error {
	log.Info().Msg("refreshing database")

	err := m.repo.DropDatabase()
	if err != nil {
		log.Error().Err(err).Msg("failed to DropDatabase (running with 'refresh' flag)")

		return fmt.Errorf("dropping database: %w", err)
	}

	log.Info().Msg("ensuring migrations table is present")

	err = m.repo.EnsureMigrationTable()
	if err != nil {
		return fmt.Errorf("failed to automatically migrate migrations table: %w", err)
	}

	return nil
}

func (m *Migrate) applyMigrations(lastAppliedMigrationNumber uint) error {
	if len(m.migrations) == 0 {
		log.Info().Msg("no migrations to apply.")

		return nil
	}

	if m.opt.VersionNumberToApply == 0 {
		m.opt.VersionNumberToApply = m.getLastMigrationNumber()
	}

	if m.opt.VersionNumberToApply < lastAppliedMigrationNumber {
		return m.applyBackwardMigrations(lastAppliedMigrationNumber)
	}

	return m.applyForwardMigrations(lastAppliedMigrationNumber)
}

func (m *Migrate) applyBackwardMigrations(lastAppliedMigrationNumber uint) error {
	m.sortMigrationsDesc()

	for _, migration := range m.migrations {
		if migration.Number > lastAppliedMigrationNumber {
			continue
		}

		if migration.Number <= m.opt.VersionNumberToApply {
			break
		}

		log.Info().Msgf("applying backwards migration %d (%s)", migration.Number, migration.Name)

		if err := m.repo.BackwardMigration(migration); err != nil {
			return fmt.Errorf("failed to apply the migration (BackwardMigration): %w", err)
		}
	}

	return nil
}

func (m *Migrate) applyForwardMigrations(lastAppliedMigrationNumber uint) error {
	m.sortMigrationsAsc()

	for _, migration := range m.migrations {
		if migration.Number <= lastAppliedMigrationNumber {
			continue
		}

		if migration.Number > m.opt.VersionNumberToApply && m.opt.VersionNumberToApply != 0 {
			break
		}

		log.Info().Msgf("applying forward migration %d (%s)", migration.Number, migration.Name)

		if err := m.repo.ForwardMigration(migration); err != nil {
			return fmt.Errorf("failed to apply the migration (ForwardMigration): %w", err)
		}
	}

	return nil
}

func (m *Migrate) sortMigrationsAsc() {
	sort.SliceStable(m.migrations, func(i, j int) bool {
		return m.migrations[i].Number < m.migrations[j].Number
	})
}

func (m *Migrate) sortMigrationsDesc() {
	sort.SliceStable(m.migrations, func(i, j int) bool {
		return m.migrations[i].Number > m.migrations[j].Number
	})
}

func (m *Migrate) getLastMigrationNumber() uint {
	var lastNumber uint

	for i := range m.migrations {
		if m.migrations[i].Number > lastNumber {
			lastNumber = m.migrations[i].Number
		}
	}

	return lastNumber
}
