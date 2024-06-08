package lib

import "sync"

type KVStore struct {
	mu sync.RWMutex
	m  map[string]string
}

func NewKVStore() *KVStore {
	return &KVStore{
		m: make(map[string]string),
	}
}

func (k *KVStore) Get(key string) string {
	k.mu.RLock()
	defer k.mu.RUnlock()
	value, ok := k.m[key]
	if !ok {
		return ""
	}
	return value
}

func (k *KVStore) Set(key, value string) (success bool) {
	defer func() {
		if r := recover(); r != nil {
			success = false
		}
	}()

	k.mu.Lock()
	defer k.mu.Unlock()

	k.m[key] = value
	return true
}

func (k *KVStore) Delete(key string) string {
	k.mu.Lock()
	defer k.mu.Unlock()
	value, ok := k.m[key]
	if !ok {
		return ""
	}
	delete(k.m, key)
	return value
}

func (k *KVStore) Append(key, value string) (success bool) {
	defer func() {
		if r := recover(); r != nil {
			success = false
		}
	}()

	k.mu.Lock()
	defer k.mu.Unlock()

	k.m[key] += value
	return true
}

func (k *KVStore) Len(key string) int {
	k.mu.RLock()
	defer k.mu.RUnlock()
	value, ok := k.m[key]
	if !ok {
		return 0
	}
	return len(value)
}
