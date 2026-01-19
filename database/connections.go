package database

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
)

// SavedConnection holds a saved database connection
type SavedConnection struct {
	Name     string           `json:"name"`
	Config   ConnectionConfig `json:"config"`
}

// ConnectionStore manages saved connections
type ConnectionStore struct {
	Connections []SavedConnection `json:"connections"`
	filePath    string
	mu          sync.RWMutex
}

// NewConnectionStore creates a new connection store
func NewConnectionStore() (*ConnectionStore, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	configDir := filepath.Join(homeDir, ".syncforge")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return nil, err
	}

	store := &ConnectionStore{
		filePath: filepath.Join(configDir, "connections.json"),
	}

	if err := store.load(); err != nil {
		return nil, err
	}
	return store, nil
}

// load reads connections from file
func (s *ConnectionStore) load() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := os.ReadFile(s.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			s.Connections = []SavedConnection{}
			return nil
		}
		return err
	}

	return json.Unmarshal(data, &s.Connections)
}

// save writes connections to file
func (s *ConnectionStore) save() error {
	data, err := json.MarshalIndent(s.Connections, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.filePath, data, 0600)
}

// GetAll returns all saved connections
func (s *ConnectionStore) GetAll() []SavedConnection {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]SavedConnection, len(s.Connections))
	copy(result, s.Connections)
	return result
}

// Save adds or updates a connection
func (s *ConnectionStore) Save(conn SavedConnection) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Check if connection with same name exists
	for i, c := range s.Connections {
		if c.Name == conn.Name {
			s.Connections[i] = conn
			return s.save()
		}
	}

	// Add new connection
	s.Connections = append(s.Connections, conn)
	return s.save()
}

// Delete removes a connection by name
func (s *ConnectionStore) Delete(name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i, c := range s.Connections {
		if c.Name == name {
			s.Connections = append(s.Connections[:i], s.Connections[i+1:]...)
			return s.save()
		}
	}
	return nil
}
