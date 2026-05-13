package database_tables

import (
	"time"

	"github.com/AdnanBinPulok/pggo"
)

type VaultTable struct {
	Table pggo.Table[VaultModel]
}

type VaultModel struct {
	ID                int64      `db:"id"`
	OwnerID           int64      `db:"owner_id"`
	Name              string     `db:"name"`
	APIKey            string     `db:"api_key"`
	IsActive          bool       `db:"is_active"`
	VaultFolderName   string     `db:"vault_folder_name"`
	TotalFiles        int64      `db:"total_files"`
	TotalSizeBytes    int64      `db:"total_size_bytes"`
	StorageBytesLimit int64      `db:"storage_bytes_limit"` // optional max storage bytes for this vault, if 0 then no vault specific limit
	UpdatedAtUTC      *time.Time `db:"updated_at"`
	CreatedAtUTC      time.Time  `db:"created_at"`
}

func NewVaultTable(conn *pggo.DatabaseConnection) *VaultTable {
	VaultsTable := pggo.Table[VaultModel]{
		Name:       "vaults",
		Connection: conn,
		Columns: []pggo.Column{
			{Name: "id", DataType: *pggo.DataType.Serial().PrimaryKey()},
			{Name: "owner_id", DataType: *pggo.DataType.BigInt().NotNull()},
			{Name: "name", DataType: *pggo.DataType.Text().NotNull()},
			{Name: "api_key", DataType: *pggo.DataType.Text().Unique().NotNull()},
			{Name: "is_active", DataType: *pggo.DataType.Bool().NotNull()},
			{Name: "vault_folder_name", DataType: *pggo.DataType.Text()},
			{Name: "total_files", DataType: *pggo.DataType.BigInt().NotNull().DefaultValue("0")},
			{Name: "total_size_bytes", DataType: *pggo.DataType.BigInt().NotNull().DefaultValue("0")},
			{Name: "storage_bytes_limit", DataType: *pggo.DataType.BigInt().NotNull().DefaultValue("0")},
			{Name: "updated_at", DataType: *pggo.DataType.TimestampTz().NotNull().DefaultValue("CURRENT_TIMESTAMP")},
			{Name: "created_at", DataType: *pggo.DataType.TimestampTz().NotNull().DefaultValue("CURRENT_TIMESTAMP")},
		},
	}
	VaultsTable.SetSchemaSync(pggo.SyncOptions{SyncColumns: true, DropMissing: false, AllowDestructive: false, DryRun: false})
	return &VaultTable{Table: VaultsTable}
}

var (
	VaultsTableInstance *VaultTable
)
