package jotdb

import (
	"encoding/json"
	"errors"
	"fmt"
	"sync"

	"go.mills.io/bitcask/v2"
)

// JotDB manages local storage and retrieval of JSON documents with concurrency support and secondary indexes.
type JotDB struct {
	db            *bitcask.Bitcask
	mu            sync.RWMutex
	path          string
	indexedFields []string
}

// NewJotDB initializes a new JotDB instance at the given local path with specified indexed fields.
func NewJotDB(dbPath string, indexedFields []string) (*JotDB, error) {
	db, err := bitcask.Open(dbPath)
	if err != nil {
		return nil, err
	}
	return &JotDB{
		db:            db,
		path:          dbPath,
		indexedFields: indexedFields,
	}, nil
}

// Store stores a JSON document with the given key and updates secondary indexes.
func (j *JotDB) Store(key string, document interface{}) error {
	j.mu.Lock()
	defer j.mu.Unlock()

	tx := j.db.Transaction()
	defer tx.Discard()

	docMap, ok := document.(map[string]interface{})
	if !ok {
		return errors.New("document must be map[string]interface{}")
	}

	data, err := marshalJSON(document)
	if err != nil {
		return err
	}

	docKey := "doc:" + key
	err = tx.Put([]byte(docKey), data)
	if err != nil {
		return err
	}

	for _, field := range j.indexedFields {
		value, ok := docMap[field]
		if !ok {
			continue
		}
		valueStr := fmt.Sprintf("%v", value)
		indexKey := "index:" + field + ":" + valueStr
		current, err := tx.Get([]byte(indexKey))
		if err != nil && err != bitcask.ErrKeyNotFound {
			return err
		}
		var keyList []string
		if err == nil {
			err = json.Unmarshal(current, &keyList)
			if err != nil {
				return err
			}
		}
		found := false
		for _, k := range keyList {
			if k == key {
				found = true
				break
			}
		}
		if !found {
			keyList = append(keyList, key)
		}
		listData, err := json.Marshal(keyList)
		if err != nil {
			return err
		}
		err = tx.Put([]byte(indexKey), listData)
		if err != nil {
			return err
		}
	}

	err = tx.Commit()
	if err != nil {
		return err
	}
	return nil
}

// Retrieve retrieves a JSON document by key and unmarshals it into the provided target.
func (j *JotDB) Retrieve(key string, target interface{}) error {
	j.mu.RLock()
	defer j.mu.RUnlock()

	docKey := "doc:" + key
	data, err := j.db.Get([]byte(docKey))
	if err != nil {
		if err == bitcask.ErrKeyNotFound {
			return errors.New("document not found")
		}
		return err
	}
	return unmarshalJSON(data, target)
}

// Delete removes a JSON document by key and updates secondary indexes.
func (j *JotDB) Delete(key string) error {
	j.mu.Lock()
	defer j.mu.Unlock()

	tx := j.db.Transaction()
	defer tx.Discard()

	docKey := "doc:" + key
	data, err := tx.Get([]byte(docKey))
	if err != nil {
		if err == bitcask.ErrKeyNotFound {
			err = tx.Commit()
			if err != nil {
				return err
			}
			return nil
		}
		return err
	}

	var docMap map[string]interface{}
	err = unmarshalJSON(data, &docMap)
	if err != nil {
		return err
	}

	for _, field := range j.indexedFields {
		value, ok := docMap[field]
		if !ok {
			continue
		}
		valueStr := fmt.Sprintf("%v", value)
		indexKey := "index:" + field + ":" + valueStr
		current, err := tx.Get([]byte(indexKey))
		if err != nil && err != bitcask.ErrKeyNotFound {
			return err
		}
		if err == bitcask.ErrKeyNotFound {
			continue
		}
		var keyList []string
		err = json.Unmarshal(current, &keyList)
		if err != nil {
			return err
		}
		newList := []string{}
		for _, k := range keyList {
			if k != key {
				newList = append(newList, k)
			}
		}
		if len(newList) > 0 {
			listData, err := json.Marshal(newList)
			if err != nil {
				return err
			}
			err = tx.Put([]byte(indexKey), listData)
			if err != nil {
				return err
			}
		} else {
			err = tx.Delete([]byte(indexKey))
			if err != nil {
				return err
			}
		}
	}

	err = tx.Delete([]byte(docKey))
	if err != nil {
		return err
	}

	err = tx.Commit()
	if err != nil {
		return err
	}
	return nil
}

// Query retrieves documents by a specific indexed field and value.
func (j *JotDB) Query(field string, value interface{}) ([]map[string]interface{}, error) {
	j.mu.RLock()
	defer j.mu.RUnlock()

	found := false
	for _, f := range j.indexedFields {
		if f == field {
			found = true
			break
		}
	}
	if !found {
		return nil, errors.New("field is not indexed")
	}

	valueStr := fmt.Sprintf("%v", value)
	indexKey := "index:" + field + ":" + valueStr
	data, err := j.db.Get([]byte(indexKey))
	if err != nil {
		if err == bitcask.ErrKeyNotFound {
			return []map[string]interface{}{}, nil
		}
		return nil, err
	}

	var keyList []string
	err = json.Unmarshal(data, &keyList)
	if err != nil {
		return nil, err
	}

	var results []map[string]interface{}
	for _, k := range keyList {
		docData, err := j.db.Get([]byte("doc:" + k))
		if err != nil {
			continue
		}
		var doc map[string]interface{}
		err = unmarshalJSON(docData, &doc)
		if err != nil {
			continue
		}
		results = append(results, doc)
	}
	return results, nil
}

// BatchStore stores multiple JSON documents with their keys in a single transaction.
func (j *JotDB) BatchStore(keys []string, documents []interface{}) error {
	if len(keys) != len(documents) {
		return errors.New("keys and documents must have the same length")
	}
	j.mu.Lock()
	defer j.mu.Unlock()

	tx := j.db.Transaction()
	defer tx.Discard()

	for i, key := range keys {
		document := documents[i]
		docMap, ok := document.(map[string]interface{})
		if !ok {
			return errors.New("document must be map[string]interface{}")
		}

		data, err := marshalJSON(document)
		if err != nil {
			return err
		}

		docKey := "doc:" + key
		err = tx.Put([]byte(docKey), data)
		if err != nil {
			return err
		}

		for _, field := range j.indexedFields {
			value, ok := docMap[field]
			if !ok {
				continue
			}
			valueStr := fmt.Sprintf("%v", value)
			indexKey := "index:" + field + ":" + valueStr
			current, err := tx.Get([]byte(indexKey))
			if err != nil && err != bitcask.ErrKeyNotFound {
				return err
			}
			var keyList []string
			if err == nil {
				err = json.Unmarshal(current, &keyList)
				if err != nil {
					return err
				}
			}
			found := false
			for _, k := range keyList {
				if k == key {
					found = true
					break
				}
			}
			if !found {
				keyList = append(keyList, key)
			}
			listData, err := json.Marshal(keyList)
			if err != nil {
				return err
			}
			err = tx.Put([]byte(indexKey), listData)
			if err != nil {
				return err
			}
		}
	}

	err := tx.Commit()
	if err != nil {
		return err
	}
	return nil
}

// BatchRetrieve retrieves multiple JSON documents by their keys.
func (j *JotDB) BatchRetrieve(keys []string) ([]map[string]interface{}, error) {
	j.mu.RLock()
	defer j.mu.RUnlock()

	var results []map[string]interface{}
	for _, key := range keys {
		docKey := "doc:" + key
		data, err := j.db.Get([]byte(docKey))
		if err != nil {
			if err == bitcask.ErrKeyNotFound {
				continue
			}
			return nil, err
		}
		var doc map[string]interface{}
		err = unmarshalJSON(data, &doc)
		if err != nil {
			return nil, err
		}
		results = append(results, doc)
	}
	return results, nil
}

// Close shuts down the JotDB instance and releases resources.
func (j *JotDB) Close() error {
	j.mu.Lock()
	defer j.mu.Unlock()
	return j.db.Close()
}
