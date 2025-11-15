# Progress Bar Implementation

This document describes the progress bar implementation for GitHub download and upload operations in Ivaldi VCS.

## Overview

Progress bars have been implemented for all major GitHub operations to provide users with real-time feedback during long-running operations like cloning repositories, downloading files, and uploading commits.

## Implementation Details

### Progress Bar Library

- **Library**: `github.com/schollz/progressbar/v3`
- **Location**: Added to `go.mod` as a dependency
- **Wrapper Package**: `internal/progress/progress.go`

The wrapper package provides Ivaldi-specific styling and convenience methods for creating consistent progress bars throughout the application.

### Progress Bar Types

1. **Download Progress Bar** - For downloading files and commits
2. **Upload Progress Bar** - For uploading files to GitHub
3. **Spinner** - For operations with unknown duration (future use)

## Features Implemented

### 1. Commit History Download (`internal/github/sync.go`)

**Location**: `importCommitHistory` function (lines 154-312)

**Features**:
- Progress bar shows current commit being processed out of total
- Updates in real-time as commits are downloaded and converted
- Displays completion status

**Example Output**:
```
Processing commits [=========>          ] 45/100 45% [0s:1s]
```

### 2. File Downloads (`internal/github/sync.go`)

**Location**: `downloadFiles` function (lines 429-548)

**Features**:
- Shows number of files downloaded vs total
- Updates dynamically with parallel downloads (up to 32 workers)
- Displays download rate and percentage

**Example Output**:
```
Downloading files [==================>  ] 234/250 94% [1m:5s]
```

### 3. File Uploads (`internal/github/sync.go`)

**Location**: `createBlobsParallel` function (lines 819-920)

**Features**:
- Visual progress for parallel blob uploads
- Shows files uploaded vs total
- Real-time updates during concurrent uploads (up to 32 workers)

**Example Output**:
```
Uploading files [===============>     ] 67/82 82% [12s:3s]
```

### 4. Initial Repository Upload (`internal/github/sync.go`)

**Location**: `PushCommit` function for empty repositories (lines 1071-1114)

**Features**:
- Progress bar for initial commit to empty repository
- Uses GitHub Contents API for first-time uploads

**Example Output**:
```
Uploading initial files [==============>  ] 15/18 83% [5s:1s]
```

## Full History Download Fix

### Problem

The original implementation had potential issues with large repositories where not all commits might be fetched due to pagination limits.

### Solution

**Location**: `internal/github/client.go` - `ListCommits` function (lines 737-805)

**Improvements**:
1. Added progress logging for multi-page fetches
2. Ensured pagination continues until all commits are retrieved
3. Added clear messaging about full history vs depth-limited history
4. Prints current page number for large repositories

**Example Output**:
```
Fetching commit history (depth: full history)...
Fetching commits: 1500 commits retrieved (page 15)...
Retrieved complete history: 1523 commits
```

## User-Facing Changes

### Command Line Flags

#### Download Command

```bash
ivaldi download <owner/repo> [flags]
```

Flags:
- `--depth N` - Limit commit history depth (0 for full history, default: 0)
- `--skip-history` - Download only latest snapshot without commit history
- `--include-tags` - Include tags and releases in the import
- `--recurse-submodules` - Automatically clone and convert Git submodules (default: true)

#### Upload Command

```bash
ivaldi upload [branch] [flags]
```

Flags:
- `--force` - Force push to remote (overwrites remote history - use with caution!)

### Visual Progress Indicators

All GitHub operations now show:
- ✓ Current progress (e.g., "45/100")
- ✓ Percentage complete
- ✓ Visual progress bar with ASCII graphics
- ✓ Elapsed time
- ✓ Estimated time remaining (when available)

## Performance Optimizations

### Parallel Operations

1. **File Downloads**: Up to 32 concurrent downloads based on file count
   - 8 workers for < 100 files
   - 16 workers for 100-500 files
   - 32 workers for > 500 files

2. **File Uploads**: Up to 32 concurrent uploads based on file count
   - 8 workers for < 50 files
   - 16 workers for 50-200 files
   - 32 workers for > 200 files

### Delta Optimization

- Upload operations detect changed files and only upload differences
- Reuses parent tree SHA when available for efficient uploads
- Skips unchanged files during downloads

### Rate Limit Handling

- Automatic detection and waiting when GitHub API limits are reached
- Progress bars pause during rate limit waits

## Documentation Updates

### Updated Documentation Files

1. `docs/commands/download.md` - Added progress bar details and full history information
2. `docs/commands/upload.md` - Added progress tracking and performance features

### New Information Added

- Progress tracking features
- Performance characteristics
- Full history download guarantees
- Parallel operation details
- Rate limit handling

## Testing

### Compilation

All changes compile successfully:
```bash
go build -o /tmp/ivaldi-test
```

### Manual Testing Recommended

To verify progress bars work correctly:

1. **Download Test**:
```bash
ivaldi download javanhut/IvaldiVCS test-repo
```

2. **Upload Test**:
```bash
cd test-repo
ivaldi gather .
ivaldi seal "Test commit"
ivaldi upload
```

## Future Enhancements

Potential improvements for future versions:

1. **Download Speed Display** - Show KB/s or MB/s for file downloads
2. **Total Size Display** - Show total bytes to download
3. **Spinner for Unknown Operations** - Use spinner for indefinite operations
4. **Color-Coded Progress** - Different colors for different stages
5. **Detailed Error Reporting** - Better error messages within progress context

## Code Quality

### No Breaking Changes

- All existing functionality preserved
- Progress bars are additive features only
- Backward compatible with existing workflows

### Clean Implementation

- Progress bar logic isolated in `internal/progress` package
- Minimal changes to existing functions
- Easy to disable/modify progress bars in future

## Summary

The progress bar implementation provides users with clear, real-time feedback during GitHub operations. Combined with the full history download fix, users can now confidently download entire repositories and see exactly what's happening at each stage of the process.
