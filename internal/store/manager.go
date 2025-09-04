package store

import (
	"fmt"
	"path/filepath"
	"sync"
)

// Manager provides shared database access to prevent locking conflicts.
type Manager struct {
	mu     sync.RWMutex
	db     *DB
	dbPath string
	refs   int // Reference count
}

// globalManager is a singleton database manager
var globalManager *Manager
var managerMu sync.Mutex

// GetSharedDB returns a shared database connection for the given Ivaldi directory.
// Multiple calls with the same ivaldiDir will return the same connection.
// The connection is reference counted and will be closed when all references are released.
func GetSharedDB(ivaldiDir string) (*SharedDB, error) {
	managerMu.Lock()
	defer managerMu.Unlock()
	
	dbPath := filepath.Join(ivaldiDir, "objects.db")
	
	// If no manager exists or it's for a different database, create a new one
	if globalManager == nil || globalManager.dbPath != dbPath {
		// Close existing manager if it exists
		if globalManager != nil {
			globalManager.close()
		}
		
		db, err := Open(dbPath)
		if err != nil {
			return nil, fmt.Errorf("open database: %w", err)
		}
		
		globalManager = &Manager{
			db:     db,
			dbPath: dbPath,
			refs:   0,
		}
	}
	
	// Increment reference count
	globalManager.refs++
	
	return &SharedDB{
		manager: globalManager,
		DB:      globalManager.db,
	}, nil
}

// SharedDB wraps a database connection with reference counting.
type SharedDB struct {
	manager *Manager
	*DB
}

// Close decrements the reference count and closes the underlying database
// when no more references exist.
func (sdb *SharedDB) Close() error {
	if sdb.manager == nil {
		return nil
	}
	
	managerMu.Lock()
	defer managerMu.Unlock()
	
	sdb.manager.refs--
	
	// If no more references, close the underlying database
	if sdb.manager.refs <= 0 {
		err := sdb.manager.close()
		globalManager = nil
		return err
	}
	
	return nil
}

// close closes the underlying database connection (internal use only)
func (m *Manager) close() error {
	if m.db != nil {
		return m.db.Close()
	}
	return nil
}