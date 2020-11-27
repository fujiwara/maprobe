package maprobe

import (
	"encoding/json"
	"sync"
)

var findHostsCache sync.Map

func cacheKey(v interface{}) (string, error) {
	key, err := json.Marshal(v)
	return string(key), err
}
