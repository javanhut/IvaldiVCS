# Seal Names - Human-Friendly Commit Identification

## Overview

Ivaldi VCS automatically generates unique, memorable names for every commit (called "seals") instead of requiring users to remember cryptographic hashes. This makes the version control experience more intuitive and user-friendly.

## Name Format

Seal names follow a consistent 4-word pattern:
```
adjective-noun-verb-adverb-hash8
```

### Examples
- `swift-eagle-flies-high-447abe9b`
- `gentle-lion-speaks-bold-e54d3630`
- `pure-moon-finds-hard-7bb30c93`

### Components
1. **Adjective**: Descriptive quality (swift, gentle, pure, etc.)
2. **Noun**: Object or entity (eagle, lion, moon, etc.)
3. **Verb**: Action (flies, speaks, finds, etc.)
4. **Adverb**: Manner (high, bold, hard, etc.)
5. **Hash8**: 8-character hex from BLAKE3 hash for uniqueness

## How It Works

### Deterministic Generation
- Same BLAKE3 hash always produces the same seal name
- Uses cryptographic hash as seed for word selection
- Ensures reproducibility across different machines

### Word Selection
The system includes carefully curated word lists:
- **32 adjectives**: swift, brave, bold, clever, mighty, gentle, wise, etc.
- **32 nouns**: eagle, mountain, river, falcon, wolf, bear, storm, etc.
- **32 verbs**: flies, runs, leaps, soars, dives, climbs, swims, etc.
- **32 adverbs**: high, fast, slow, well, far, near, deep, etc.

This provides over 1 million unique combinations before considering the hash suffix.

## Usage

### Creating Seals
```bash
# Normal seal creation
$ ivaldi seal "Add user authentication"
Created seal: swift-eagle-flies-high-447abe9b (447abe9b)

# The system automatically generates the name
# No user input required for naming
```

### Viewing Seals
```bash
# List all seals
$ ivaldi seals list
Seals in repository (3 total):

pure-moon-finds-hard-7bb30c93             7bb30c93 (2 hours ago)
  "Update files and add world.txt"

swift-eagle-flies-high-447abe9b           447abe9b (1 day ago)
  "Add user authentication"

gentle-lion-speaks-bold-e54d3630          e54d3630 (2 days ago)
  "Initial commit"
```

### Referencing Seals
Seals can be referenced in multiple ways:

```bash
# Full name
ivaldi seals show swift-eagle-flies-high-447abe9b

# Name prefix (without hash)
ivaldi seals show swift-eagle-flies-high

# Partial name
ivaldi seals show swift-eagle

# Hash prefix
ivaldi seals show 447a
ivaldi seals show 447abe9b
```

### Whereami Integration
The `whereami` command displays seal names:

```bash
$ ivaldi wai
Timeline: main
Type: Local Timeline
Last Seal: swift-eagle-flies-high-447abe9b (1 day ago)
Message: "Add user authentication"
Remote: origin/main (up to date)
Workspace: Clean
```

## Benefits

### 1. Memorability
- "swift-eagle-flies-high" is much easier to remember than "447abe9b"
- Natural language patterns are more intuitive for humans
- Reduces cognitive load when working with version history

### 2. Uniqueness
- 4-word combination provides excellent entropy
- Hash suffix guarantees uniqueness even with identical word combinations
- Collision probability is effectively zero

### 3. Searchability
- Partial matching works with any component
- Tab completion can be implemented for shell integration
- Grep-friendly for scripting and automation

### 4. Communication
- Teams can reference commits in natural language
- "Check out the swift-eagle commit" vs "Check out 447abe9b"
- Better for documentation and issue tracking

## Implementation Details

### Storage Structure
```
.ivaldi/
└── refs/
    └── seals/
        ├── swift-eagle-flies-high-447abe9b
        ├── gentle-lion-speaks-bold-e54d3630
        └── pure-moon-finds-hard-7bb30c93
```

Each file contains:
```
447abe9bcee3307bfbd3acc4667d7393ed2441f984ed7bf0826d05b78e068cab 1694012345 Add user authentication
```

Format: `full_hash timestamp message`

### Name Resolution Algorithm
1. Try exact seal name match
2. Try partial name matching (prefix)
3. Try hash matching (prefix)
4. Return error if no match or multiple matches

### Backwards Compatibility
- Hash-based references continue to work
- Existing tools can still use traditional hash lookups
- Gradual migration path for existing repositories

## Command Reference

### `ivaldi seal <message>`
Creates a new seal with auto-generated name:
```bash
$ ivaldi seal "Fix authentication bug"
Created seal: brave-falcon-dives-deep-a1b2c3d4 (a1b2c3d4)
```

### `ivaldi seals list`
Lists all seals with names and metadata:
```bash
$ ivaldi seals list
Seals in repository (2 total):
...
```

### `ivaldi seals show <reference>`
Shows detailed information about a seal:
```bash
$ ivaldi seals show brave-falcon
Seal: brave-falcon-dives-deep-a1b2c3d4
Hash: a1b2c3d4cee3307bfbd3acc4667d7393ed2441f984ed7bf0826d05b78e068cab
Short Hash: a1b2c3d4
Created: 2025-01-20 15:30:00 (2 hours ago)
Message: Fix authentication bug
```

## Best Practices

### 1. Use Descriptive Commit Messages
Even with memorable seal names, good commit messages remain important:
```bash
# Good
ivaldi seal "Fix user authentication timeout issue"

# Less helpful
ivaldi seal "Fix bug"
```

### 2. Reference by Prefix
Use the shortest unique prefix for communication:
```bash
# Instead of full name
swift-eagle-flies-high-447abe9b

# Use prefix
swift-eagle
```

### 3. Scripting
Scripts can use either seal names or hashes:
```bash
#!/bin/bash
# Both work
COMMIT_REF="swift-eagle-flies-high"
COMMIT_REF="447abe9b"
```

## Future Enhancements

### Planned Features
1. **Custom Name Patterns**: Allow user-defined naming schemes
2. **Themed Word Lists**: Domain-specific vocabularies (colors, animals, etc.)
3. **Shell Completion**: Tab completion for seal names
4. **Timeline Integration**: Reference seals in timeline operations

### Integration Opportunities
1. **GitHub Integration**: Map seal names to GitHub commit SHAs
2. **Issue Tracking**: Reference seals in issue comments naturally
3. **Documentation**: Generate readable changelogs with seal names
4. **Code Review**: Discuss specific commits using memorable names

The seal naming system makes Ivaldi VCS more approachable and user-friendly while maintaining all the cryptographic integrity and performance benefits of the underlying hash-based system.