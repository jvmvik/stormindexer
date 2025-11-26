# StormIndexer

A powerful file indexing and syncing tool written in Go. StormIndexer allows you to index files across multiple disks, machines, and external drives, track file metadata, calculate checksums, and synchronize files between different locations.

## Features

- **Fast File Indexing**: Quickly scan and index large directory structures
- **Checksum Calculation**: Optional SHA256 checksums for duplicate detection
- **Cross-Machine Sync**: Compare and sync files across different machines and drives
- **File Synchronization**: Uses rsync under the hood for efficient file copying
- **Change Detection**: Track file additions, updates, and deletions
- **Duplicate Detection**: Find duplicate files across all indexed locations
- **SQLite Database**: Lightweight, portable database storage
- **CLI Interface**: Easy-to-use command-line interface

## Installation

### Prerequisites

- Go 1.21 or later
- rsync (for file synchronization features)

### Build from Source

```bash
git clone <repository-url>
cd stormindexer
go mod download
go build -o stormindexer
```

## Usage

### Index a Directory

Index all files in a directory:

```bash
# Basic indexing
./stormindexer index /path/to/directory

# Index with checksums (slower but enables duplicate detection)
./stormindexer index /path/to/directory --checksums

# Index with a custom name
./stormindexer index /path/to/directory --name "My External Drive"

# Force reindex even if index exists
./stormindexer index /path/to/directory --force
```

### List Indexes

View all indexed locations:

```bash
./stormindexer list
```

### List Files in an Index

View all files in a specific index:

```bash
./stormindexer list files <index-id>
```

### Reindex

Update an existing index to reflect changes:

```bash
./stormindexer reindex <index-id>
./stormindexer reindex <index-id> --checksums
```

### Remove an Index

Remove an indexed directory from the database:

```bash
# Show what will be removed (requires --force to actually remove)
./stormindexer remove <index-id>

# Actually remove the index (skips confirmation)
./stormindexer remove <index-id> --force
```

**Note**: This only removes the index from the database. It does NOT delete the actual files on disk.

### Compare Indexes

Compare two indexes to see differences:

```bash
./stormindexer compare <index-id-1> <index-id-2>
```

### Sync Indexes

Sync files from one index to another using rsync:

```bash
# Dry run (show what would be synced)
./stormindexer sync <source-index-id> <target-index-id> --dry-run

# Actual sync (copies files using rsync)
./stormindexer sync <source-index-id> <target-index-id>

# Sync and delete extra files in target (use with caution!)
./stormindexer sync <source-index-id> <target-index-id> --delete
```

**Note**: The sync command uses `rsync` to perform actual file copying. It preserves file permissions, timestamps, and other metadata. The `--delete` flag will remove files in the target that don't exist in the source, making the target an exact mirror.

### Find Duplicates

Find duplicate files across all indexes:

```bash
./stormindexer duplicates
```

### Database Statistics

Show database file location, size, and statistics:

```bash
./stormindexer stat
```

This displays:
- Database file path and size on disk
- Total number of indexes
- Total files indexed across all indexes
- Total size of indexed files
- Per-index breakdown with file counts and sizes

## Configuration

StormIndexer uses a configuration file located at `~/.stormindexer/config.yaml`. You can also create a `config.yaml` in the current directory.

Example configuration:

```yaml
database_path: ".stormindexer.db"
machine_id: "my-computer"
```

## Database

By default, StormIndexer stores its database in `.stormindexer.db` in the current directory. You can change this in the configuration file.

## Use Cases

1. **Backup Verification**: Index your backup drives and compare with source to ensure everything is backed up
2. **Duplicate Cleanup**: Find duplicate files across multiple drives
3. **File Synchronization**: Keep files in sync across multiple machines or external drives
4. **Change Tracking**: Monitor changes in directory structures over time
5. **Archive Management**: Index and track files across multiple archive locations

## Examples

### Index Multiple External Drives

```bash
# Index first drive
./stormindexer index /Volumes/Drive1 --name "Backup Drive 1" --checksums

# Index second drive
./stormindexer index /Volumes/Drive2 --name "Backup Drive 2" --checksums

# Compare them
./stormindexer compare <index-id-1> <index-id-2>
```

### Sync Files Between Machines

```bash
# On machine 1: Index the source directory
./stormindexer index /home/user/documents --name "Machine1 Documents" --checksums

# On machine 2: Index the target directory
./stormindexer index /home/user/documents --name "Machine2 Documents" --checksums

# Copy the database file to machine 2, then compare
./stormindexer compare <machine1-index-id> <machine2-index-id>
```

## Project Structure

```
stormindexer/
├── cmd/           # CLI commands
├── internal/
│   ├── config/    # Configuration management
│   ├── database/  # Database layer
│   ├── indexer/   # File indexing engine
│   ├── models/    # Data models
│   └── sync/      # Synchronization engine
├── main.go        # Entry point
└── go.mod         # Go module definition
```

## License

MIT License

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

