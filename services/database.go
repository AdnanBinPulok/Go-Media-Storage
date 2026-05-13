package services

import (
	"fmt"
	"go-media-storage/database_tables"
	"go-media-storage/settings"
	"sync"

	"github.com/AdnanBinPulok/pggo"
)

var (
	initDBOnce   sync.Once
	initDBErr    error
	DatabaseConn *pggo.DatabaseConnection
)

func InitDatabase() error {
	initDBOnce.Do(func() {
		cfg := settings.GetConfig().DATABASE
		conn := &pggo.DatabaseConnection{
			DBURL:          cfg.DSN,
			MaxConnections: cfg.MaxOpenConns,
			Reconnect:      true,
		}
		DatabaseConn = conn

		if _, err := conn.Connect(); err != nil {
			initDBErr = fmt.Errorf("connect postgres: %w", err)
			return
		}

		if err := conn.Ping(); err != nil {
			conn.Close()
			initDBErr = fmt.Errorf("ping postgres: %w", err)
			return
		}

		database_tables.UsersTableInstance = database_tables.NewUserTable(conn)
		database_tables.VaultsTableInstance = database_tables.NewVaultTable(conn)
		database_tables.SessionsTableInstance = database_tables.NewSessionTable(conn)
		database_tables.MediaFilesTableInstance = database_tables.NewMediaTable(conn)

		if err := database_tables.UsersTableInstance.Table.CreateTable(); err != nil {
			initDBErr = fmt.Errorf("create users table: %w", err)
			return
		}
		if err := database_tables.VaultsTableInstance.Table.CreateTable(); err != nil {
			initDBErr = fmt.Errorf("create vaults table: %w", err)
			return
		}
		if err := database_tables.SessionsTableInstance.Table.CreateTable(); err != nil {
			initDBErr = fmt.Errorf("create sessions table: %w", err)
			return
		}
		if err := database_tables.MediaFilesTableInstance.Table.CreateTable(); err != nil {
			initDBErr = fmt.Errorf("create media_files table: %w", err)
			return
		}
	})

	return initDBErr
}
