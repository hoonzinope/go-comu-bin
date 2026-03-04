package application

type Cache interface {
	Get(key string) (interface{}, bool)
	Set(key string, value interface{})
	SetWithTTL(key string, value interface{}, ttlSeconds int)
	Delete(key string)
}
