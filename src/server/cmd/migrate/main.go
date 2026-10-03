package main

import (
	"errors"
	"fmt"
	"os"
	"social/src/server/internal/platform/config"
	"social/src/server/pkg/db/sqlite"
	"strconv"

	"github.com/golang-migrate/migrate/v4"
)

const usage = `usage: go run ./src/server/cmd/migrate <commands>

	up			apply all pending operations
	down		roll back all migrations; empty the db
	step N		apply N (N>0) or roll back (N<0) migrations
	force V		mark version V as clean without running anything
	version		print the current version and 'dirty' if dirty
`

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "migrare:", err)
		os.Exit(1)
	}
}

func run(args []string) error {

	if len(args) == 0 { 
		return fmt.Errorf(usage)
	}

	// here, we're going to use the same config and Open()
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	db, err := sqlite.Open(cfg.DBPath)
	if err != nil {
		return err
	}

	m, err := sqlite.NewMigrator(db, cfg.DBPath)
	if err != nil {
		db.Close()
		return err
	}

	switch args[0] {
	case "up":
		err = m.Up()
	case "down":
		err = m.Down()
	case "steps":
		n, stepsErr := intArg(args)
		if stepsErr != nil {
			return stepsErr
		}
		err = m.Steps(n)
	case "force":
		v, forceErr := intArg(args)
		if forceErr != nil {
			return forceErr
		}
		err = m.Force(v)
	case "version":
		v, dirty, verr := m.Version()
		if errors.Is(verr, migrate.ErrNilVersion) {
			fmt.Println("Error: no migration applied")
			return nil
		}
		if verr != nil {
			return verr
		}
		fmt.Printf("version %d, dirty=%v\n", v, dirty)
		return nil
	default:
		// uncaught errors
		return errors.New(usage)
	}

	if errors.Is(err, migrate.ErrNoChange) {
		fmt.Println("no change")
		return nil
	}

	if err != nil {
		return err
	}

	// works
	fmt.Println("ok")
	return nil
}

// if two is greater than the len of args, return NOTHING
func intArg(args []string) (int, error) {
	if len(args) < 2 {
		return 0, nil
	}
	// genuinely didnt know this was possible, to return a somethign that already returns two things
	return strconv.Atoi(args[1])
}