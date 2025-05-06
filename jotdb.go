package jotdb

import (
	"errors"
	"github.com/prologic/bitcask"
	"sync"
)

// JotDB manages local storage and retrieval of JSON documents with concurrency support.
type JotDB struct {
	db   *bitcask.Bitcask
	mu   sync.RWMutex
	path string
}

// NewJotDB initializes a new JotDB instance at the given local path.
func NewJotDB(dbPath string) (*JotDB, error) {
	db, err := bitcask.Open(dbPath,
	 // bitcask.WithMaxDataFileSize(100<<20), // 100MB per file
		bitcask.WithSync(true),               // Ensure durability
	)
	if err != nil {
		return nil, err
	}
	return &JotDB{
		db:   db,
		path: dbPath,
	}, nil
}

// Store stores a JSON document with the given key.
func (j *JotDB) Store(key string, document interface{}) error {
	j.mu.Lock()
	defer j.mu.Unlock()

	data, err := marshalJSON(document)
	if err != nil {
		return err
	}
	return j.db.Put([]byte(key), data)
}

// Retrieve retrieves a JSON document by key and unmarshals it into the provided target.
func (j *JotDB) Retrieve(key string, target interface{}) error {
	j.mu.RLock()
	defer j.mu.RUnlock()

	data, err := j.db.Get([]byte(key))
	if err != nil {
		if errors.Is(err, bitcask.ErrKeyNotFound) {
			return errors.New("document not found")
		}
		return err
	}

	return unmarshalJSON(data, target)
}

// Delete removes a JSON document by key.
func (j *JotDB) Delete(key string) error {
	j.mu.Lock()
	defer j.mu.Unlock()

	return j.db.Delete([]byte(key))
}

// Close shuts down the JotDB instance and releases resources.
func (j *JotDB) Close() error {
	j.mu.Lock()
	defer j.mu.Unlock()
	return j.db.Close()
}