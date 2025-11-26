# Testing Guide

This document describes the test suite for StormIndexer.

## Running Tests

### Run all tests
```bash
go test ./...
```

### Run tests with verbose output
```bash
go test ./... -v
```

### Run tests with coverage
```bash
go test ./... -cover
```

### Run tests for a specific package
```bash
go test ./internal/database/...
go test ./internal/indexer/...
go test ./internal/sync/...
go test ./internal/models/...
go test ./internal/config/...
```

### Run a specific test
```bash
go test ./internal/database/... -run TestCreateIndex
```

## Test Coverage

Current test coverage:
- **models**: 100.0% - All model functions tested
- **database**: 90.9% - Core database operations tested
- **indexer**: 74.2% - File indexing functionality tested
- **config**: 63.0% - Configuration loading tested
- **sync**: 44.3% - Sync comparison logic tested (rsync execution not tested)

## Test Structure

### Unit Tests

#### `internal/models/file_test.go`
Tests for file model functions:
- `TestCalculateChecksum` - Verifies checksum calculation
- `TestCalculateChecksum_NonExistentFile` - Error handling
- `TestCalculateChecksum_DifferentContent` - Uniqueness verification

#### `internal/database/database_test.go`
Tests for database operations:
- `TestNewDB` - Database initialization
- `TestCreateIndex` - Index creation
- `TestGetIndex_NotFound` - Error handling
- `TestListIndexes` - Listing all indexes
- `TestUpsertFile` - File insertion/update
- `TestUpsertFile_Update` - File update verification
- `TestListFiles` - File listing
- `TestDeleteFile` - File deletion
- `TestUpdateIndexStats` - Statistics calculation
- `TestFindFilesByChecksum` - Duplicate detection

#### `internal/indexer/indexer_test.go`
Tests for file indexing:
- `TestNewIndexer` - Indexer initialization
- `TestIndex_Basic` - Basic indexing functionality
- `TestIndex_WithChecksums` - Indexing with checksum calculation
- `TestIndex_SkipsHiddenFiles` - Hidden file filtering
- `TestReindex_AddNewFile` - Reindexing with new files
- `TestReindex_UpdateFile` - Reindexing with updated files
- `TestReindex_DeleteFile` - Reindexing with deleted files

#### `internal/sync/sync_test.go`
Tests for synchronization:
- `TestCompareIndexes_NewFiles` - Detecting new files
- `TestCompareIndexes_UpdatedFiles` - Detecting updated files
- `TestCompareIndexes_DuplicateDetection` - Duplicate file detection
- `TestFindDuplicates` - Finding duplicates across indexes
- `TestFindDuplicates_NoDuplicates` - No duplicates scenario
- `TestCompareIndexes_IdenticalIndexes` - Identical indexes comparison

#### `internal/config/config_test.go`
Tests for configuration:
- `TestLoad_Defaults` - Default configuration loading
- `TestLoad_WithConfigFile` - Configuration file loading
- `TestGetDefaultMachineID` - Machine ID generation

## Test Helpers

Tests use temporary directories and databases created with `t.TempDir()` to ensure isolation and cleanup.

## Writing New Tests

When adding new functionality:

1. **Create test file**: Add `*_test.go` file in the same package
2. **Use test helpers**: Leverage `setupTestDB`, `setupTestIndexer`, etc.
3. **Test edge cases**: Include error cases and boundary conditions
4. **Clean up**: Use `t.TempDir()` for temporary files
5. **Name tests clearly**: Use descriptive test names like `TestFunctionName_Scenario`

Example:
```go
func TestNewFeature(t *testing.T) {
    // Setup
    db, _ := setupTestDB(t)
    defer db.Close()
    
    // Test
    result := db.NewFeature()
    
    // Verify
    if result != expected {
        t.Errorf("Expected %v, got %v", expected, result)
    }
}
```

## Continuous Integration

Tests should pass before merging any pull request. Run `make test` or `go test ./...` locally before committing.

