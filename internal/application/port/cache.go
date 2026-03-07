package port

type Cache interface {
	Get(key string) (interface{}, bool)
	Set(key string, value interface{})
	SetWithTTL(key string, value interface{}, ttlSeconds int)
	Delete(key string)
	DeleteByPrefix(prefix string) int
	GetOrSetWithTTL(key string, ttlSeconds int, loader func() (interface{}, error)) (interface{}, error)
}
