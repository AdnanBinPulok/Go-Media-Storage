package database_tables

import (
	"fmt"
	"time"

	"github.com/AdnanBinPulok/pggo"
)

type MediaTable struct {
	Table pggo.Table[MediaFileModel]
}

type MediaFileModel struct {
	ID                        int64                  `db:"id"`
	OwnerID                   int64                  `db:"owner_id"`
	VaultID                   int64                  `db:"vault_id"`
	OriginalName              string                 `db:"original_name"`
	MimeType                  string                 `db:"mime_type"`
	FileType                  string                 `db:"file_type"`
	FileExt                   string                 `db:"file_ext"`
	FileSizeBytes             int64                  `db:"file_size_bytes"`
	StoredName                string                 `db:"stored_name"`
	StoragePath               string                 `db:"storage_path"`
	Compressed                bool                   `db:"compressed"`
	UncompressedFileSizeBytes *int64                 `db:"uncompressed_file_size_bytes"`
	CompressionMetadata       map[string]interface{} `db:"compression_metadata"`
	Deleted                   bool                   `db:"deleted"`
	DeletedAtUTC              *time.Time             `db:"deleted_at"`
	CreatedAtUTC              time.Time              `db:"created_at"`
}

func NewMediaTable(conn *pggo.DatabaseConnection) *MediaTable {
	MediaFilesTable := pggo.Table[MediaFileModel]{
		Name:       "media_files",
		Connection: conn,
		Columns: []pggo.Column{
			{Name: "id", DataType: *pggo.DataType.Serial().PrimaryKey()},
			{Name: "owner_id", DataType: *pggo.DataType.BigInt()},
			{Name: "vault_id", DataType: *pggo.DataType.BigInt()},
			{Name: "stored_name", DataType: *pggo.DataType.Text().Unique().NotNull()},
			{Name: "original_name", DataType: *pggo.DataType.Text().NotNull()},
			{Name: "mime_type", DataType: *pggo.DataType.Text().NotNull()},
			{Name: "file_type", DataType: *pggo.DataType.Text().NotNull()},
			{Name: "file_ext", DataType: *pggo.DataType.Text().NotNull()},
			{Name: "file_size_bytes", DataType: *pggo.DataType.BigInt().NotNull()},
			{Name: "storage_path", DataType: *pggo.DataType.Text().NotNull()},
			{Name: "compressed", DataType: *pggo.DataType.Bool().NotNull().DefaultValue("false")},
			{Name: "uncompressed_file_size_bytes", DataType: *pggo.DataType.BigInt()},
			{Name: "compression_metadata", DataType: *pggo.DataType.JSONB()},
			{Name: "deleted", DataType: *pggo.DataType.Bool().NotNull()},
			{Name: "deleted_at", DataType: *pggo.DataType.TimestampTz()},
			{Name: "created_at", DataType: *pggo.DataType.TimestampTz().NotNull().DefaultValue("CURRENT_TIMESTAMP")},
		},
	}
	MediaFilesTable.CacheKey = "stored_name"
	MediaFilesTable.EnableCache(5 * time.Second)
	MediaFilesTable.SetSchemaSync(pggo.SyncOptions{SyncColumns: true, DropMissing: true, AllowDestructive: true, DryRun: false})
	return &MediaTable{Table: MediaFilesTable}
}

func (mt *MediaTable) GetTotalStorageUsedForVault(vaultID int64) (int64, error) {
	if mt.Table.Connection == nil {
		return 0, fmt.Errorf("database connection is not initialized")
	}

	queryResult, err := mt.Table.Connection.RunQuery(
		"SELECT COALESCE(SUM(file_size_bytes), 0)::bigint AS total_size FROM media_files WHERE vault_id = $1 AND deleted = $2",
		[]any{vaultID, false},
	)
	if err != nil {
		return 0, fmt.Errorf("query total storage used for vault: %w", err)
	}

	if len(queryResult) == 0 {
		return 0, nil
	}

	totalSize, ok := queryResult[0]["total_size"].(int64)
	if !ok {
		return 0, fmt.Errorf("invalid total_size value type")
	}

	return totalSize, nil
}

var MediaFilesTableInstance *MediaTable
