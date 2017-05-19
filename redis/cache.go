package redis

import (
	"sync"
	"time"

	"github.com/hashicorp/golang-lru"
)

const (
	cacheInvalidationPeriod = 5 * time.Minute
)

type refresher func() (func(func(k interface{}, v interface{})), error)

type cache struct {
	*lru.Cache
	key       string
	refreshFn refresher
	mx        sync.RWMutex
}

func newCache(key string, maxSize int, refreshFn refresher) *cache {
	c, _ := lru.New(maxSize)
	result := &cache{
		Cache:     c,
		key:       key,
		refreshFn: refreshFn,
	}
	go result.keepFresh()
	return result
}

func (c *cache) Get(key interface{}) (interface{}, bool) {
	c.mx.RLock()
	result, found := c.Cache.Get(key)
	c.mx.RUnlock()
	return result, found
}

func (c *cache) keepFresh() {
	for {
		log.Debugf("Refreshing cache for '%v'", c.key)
		applyUpdates, err := c.refreshFn()
		if err != nil {
			log.Errorf("Unable to refresh cache for '%v' from Redis: %v", c.key, err)
		} else {
			entries := 0
			c.mx.Lock()
			c.Purge()
			applyUpdates(func(k interface{}, v interface{}) {
				c.Add(k, v)
				entries++
			})
			c.mx.Unlock()
			log.Debugf("Refreshed cache for '%v' with %d entries", c.key, entries)
		}
		time.Sleep(cacheInvalidationPeriod)
	}
}
