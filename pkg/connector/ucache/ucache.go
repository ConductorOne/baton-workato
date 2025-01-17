package ucache

type HashSet[TKey comparable, TValueKey comparable, TValue any] struct {
	cache map[TKey]map[TValueKey]*TValue
}

func NewUCache[TKey comparable, TValueKey comparable, TValue any]() *HashSet[TKey, TValueKey, TValue] {
	return &HashSet[TKey, TValueKey, TValue]{
		cache: make(map[TKey]map[TValueKey]*TValue),
	}
}

func (c *HashSet[TKey, TValueKey, TValue]) Get(key TKey, valueKey TValueKey) (*TValue, bool) {
	if value, ok := c.cache[key]; ok {
		if v, ok := value[valueKey]; ok {
			return v, true
		}
	}
	return nil, false
}

func (c *HashSet[TKey, TValueKey, TValue]) Set(key TKey, valueKey TValueKey, value *TValue) {
	if _, ok := c.cache[key]; !ok {
		c.cache[key] = make(map[TValueKey]*TValue)
	}
	c.cache[key][valueKey] = value
}

func (c *HashSet[TKey, TValueKey, TValue]) GetAll(key TKey) []*TValue {
	response := make([]*TValue, 0)

	if value, ok := c.cache[key]; ok {
		for _, v := range value {
			response = append(response, v)
		}
	}

	return response
}
