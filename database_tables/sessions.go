package database_tables

import (
	"time"

	"github.com/AdnanBinPulok/pggo"
)

type SessionTable struct {
	Table pggo.Table[SessionModel]
}

type SessionModel struct {
	ID           int64     `db:"id"`
	UserID       int64     `db:"user_id"`
	SessionToken string    `db:"session_token"`
	IPAddress    string    `db:"ip_address"`
	UserAgent    string    `db:"user_agent"`
	ExpiresAtUTC time.Time `db:"expires_at"`
	UpdatedAtUTC time.Time `db:"updated_at"`
	CreatedAtUTC time.Time `db:"created_at"`
}

func NewSessionTable(conn *pggo.DatabaseConnection) *SessionTable {
	SessionsTable := pggo.Table[SessionModel]{
		Name:       "sessions",
		Connection: conn,
		Columns: []pggo.Column{
			{Name: "id", DataType: *pggo.DataType.Serial().PrimaryKey()},
			{Name: "user_id", DataType: *pggo.DataType.BigInt().NotNull()},
			{Name: "session_token", DataType: *pggo.DataType.Text().Unique().NotNull()},
			{Name: "ip_address", DataType: *pggo.DataType.Text()},
			{Name: "user_agent", DataType: *pggo.DataType.Text()},
			{Name: "expires_at", DataType: *pggo.DataType.TimestampTz()},
			{Name: "updated_at", DataType: *pggo.DataType.TimestampTz().NotNull().DefaultValue("CURRENT_TIMESTAMP")},
			{Name: "created_at", DataType: *pggo.DataType.TimestampTz().NotNull().DefaultValue("CURRENT_TIMESTAMP")},
		},
	}
	SessionsTable.SetSchemaSync(pggo.SyncOptions{SyncColumns: true, DropMissing: false, AllowDestructive: false, DryRun: false})
	SessionsTable.CacheKey = "session_token"
	SessionsTable.EnableCache(5 * time.Second)
	return &SessionTable{Table: SessionsTable}
}

var (
	SessionsTableInstance *SessionTable
)
