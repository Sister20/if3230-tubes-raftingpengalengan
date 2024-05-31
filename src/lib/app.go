package lib

type KVStore struct {
	store map[string]string
}

func NewKVStore() *KVStore {
	return &KVStore{store: make(map[string]string)}
}
