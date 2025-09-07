# Staging Area Preservation Fix

## Issue
The staging area was not properly preserving staged files when switching between timelines. When using `ivaldi gather` to stage multiple files and then switching timelines, the staged files would be lost or incorrectly restored.

## Root Cause
The issue was in the `shelf.go` file where staged files were being read from disk using `strings.Fields()` which splits on ANY whitespace character. This caused problems when:
1. File paths contained spaces
2. Multiple files were staged and separated by newlines

## Solution
Fixed the staging file parsing in `/internal/shelf/shelf.go`:
- Changed from `strings.Fields()` to properly splitting by newlines
- Added proper trimming of whitespace while preserving file paths
- Ensured consistent newline handling when writing staged files back

## Code Changes

### Before (problematic code):
```go
if data, err := os.ReadFile(stageFile); err == nil {
    stagedFiles = strings.Fields(string(data))
}
```

### After (fixed code):
```go
if data, err := os.ReadFile(stageFile); err == nil {
    // Split by newlines to preserve file paths with spaces
    lines := strings.Split(string(data), "\n")
    for _, line := range lines {
        line = strings.TrimSpace(line)
        if line != "" {
            stagedFiles = append(stagedFiles, line)
        }
    }
}
```

## Testing
The fix was verified with the following test scenarios:
1. Staging multiple files on one timeline
2. Switching to another timeline (triggers auto-shelving)
3. Staging files on the new timeline
4. Switching back to the original timeline
5. Verifying all originally staged files are preserved

## Impact
This fix ensures that:
- Staged files are properly preserved across timeline switches
- File paths with spaces are handled correctly
- The staging area maintains consistency with auto-shelving
- Users can confidently switch between timelines without losing their staging state