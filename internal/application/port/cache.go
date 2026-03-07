package port

type Cache interface {
	Get(key string) (interface{}, bool, error)
	Set(key string, value interface{}) error
	SetWithTTL(key string, value interface{}, ttlSeconds int) error
	Delete(key string) error
	DeleteByPrefix(prefix string) (int, error)
	GetOrSetWithTTL(key string, ttlSeconds int, loader func() (interface{}, error)) (interface{}, error)
}
