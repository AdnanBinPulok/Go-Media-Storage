package database_tables

import (
	"time"

	"github.com/AdnanBinPulok/pggo"
)

type UserTable struct {
	Table pggo.Table[UserModel]
}

type UserModel struct {
	ID                int64     `db:"id"`
	FullName          string    `db:"full_name"`
	Email             string    `db:"email"`
	MaxVaults         int       `db:"vaults_limit"`
	StorageBytesLimit int64     `db:"storage_bytes_limit"`
	IsActive          bool      `db:"is_active"`
	PasswordHash      string    `db:"password_hash"`
	UpdatedAtUTC      time.Time `db:"updated_at"`
	CreatedAtUTC      time.Time `db:"created_at"`
}

func NewUserTable(conn *pggo.DatabaseConnection) *UserTable {
	UsersTable := pggo.Table[UserModel]{
		Name:       "users",
		Connection: conn,
		Columns: []pggo.Column{
			{Name: "id", DataType: *pggo.DataType.Serial().PrimaryKey()},
			{Name: "full_name", DataType: *pggo.DataType.Text().NotNull()},
			{Name: "email", DataType: *pggo.DataType.Text().Unique().NotNull()},
			{Name: "vaults_limit", DataType: *pggo.DataType.Integer().NotNull().DefaultValue("1")},
			{Name: "storage_bytes_limit", DataType: *pggo.DataType.BigInt().NotNull().DefaultValue("1073741824")}, // 1 GB default
			{Name: "is_active", DataType: *pggo.DataType.Bool().NotNull()},
			{Name: "password_hash", DataType: *pggo.DataType.Text()},
			{Name: "updated_at", DataType: *pggo.DataType.TimestampTz().NotNull().DefaultValue("CURRENT_TIMESTAMP")},
			{Name: "created_at", DataType: *pggo.DataType.TimestampTz().NotNull().DefaultValue("CURRENT_TIMESTAMP")},
		},
		DebugMode: true,
	}
	UsersTable.SetSchemaSync(pggo.SyncOptions{SyncColumns: true, DropMissing: false, AllowDestructive: false, DryRun: false})
	UsersTable.CacheKey = "id"
	UsersTable.EnableCache(5 * time.Second)
	return &UserTable{Table: UsersTable}
}

var (
	UsersTableInstance *UserTable
)
