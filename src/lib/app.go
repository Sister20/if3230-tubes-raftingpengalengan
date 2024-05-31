package lib

import "sync"

type KVStore struct {
	mu    sync.RWMutex
	store map[string]string
}

func NewKVStore() *KVStore {
	return &KVStore{store: make(map[string]string)}
}

// Get return empty string if key not found
func (k *KVStore) Get(key string) string {
	k.mu.RLock()
	defer k.mu.RUnlock()
	value, ok := k.store[key]

	if !ok {
		return ""
	}

	return value
}

func (k *KVStore) Set(key string, value string) {
	k.mu.Lock()
	defer k.mu.Unlock()
	k.store[key] = value
}

func (k *KVStore) Len(key string) int {
	k.mu.RLock()
	defer k.mu.RUnlock()
	value, ok := k.store[key]
	if !ok {
		return 0
	}
	return len(value)
}

func (k *KVStore) Delete(key string) string {
	k.mu.Lock()
	defer k.mu.Unlock()
	value, ok := k.store[key]
	if !ok {
		return ""
	}
	delete(k.store, key)
	return value
}

func (k *KVStore) Append(key string, value string) {
	k.mu.Lock()
	defer k.mu.Unlock()
	k.store[key] += value
}
