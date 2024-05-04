package util

func CreateReverseIndex() *ReverseIndex {
	return &ReverseIndex{
		hashTable: make(map[string][]*string),
	}
}

type ReverseIndex struct {
	hashTable map[string][]*string
}

type Value struct {
	Index int
	Data  []rune
}

type listMap struct {
	list    []*string
	hashMap map[string]bool
}

func (r *ReverseIndex) AddData(attribute, value string) {
	data := []rune(attribute)
	var keys []string

	var dfs func(idx int, val *Value)
	dfs = func(idx int, val *Value) {
		if idx >= len(data) ||
			(val != nil && idx-val.Index > 1) {
			if val != nil {
				keys = append(keys, string(val.Data))
			}
			return
		}
		if val == nil {
			dfs(idx+1, &Value{Index: idx, Data: []rune{data[idx]}})
			dfs(idx+1, val)
		} else {
			dfs(idx+1, &Value{Index: idx, Data: append(val.Data, data[idx])})
			dfs(idx+1, val)
		}
	}
	dfs(0, nil)

	for _, key := range keys {
		r.hashTable[key] = append(r.hashTable[key], &value)
	}
}

func (r *ReverseIndex) Search(value string) []*string {
	return r.hashTable[value]
}

func (r *ReverseIndex) FuzzSearch(value string) []*string {
	data := []rune(value)
	res := listMap{
		hashMap: make(map[string]bool),
	}

	var dfs func(idx int, val *Value)
	dfs = func(idx int, val *Value) {
		if idx >= len(data) ||
			(val != nil && idx-val.Index > 1) {
			if val != nil {
				key := string(val.Data)
				for _, val := range r.hashTable[key] {
					val := val
					if _, ok := res.hashMap[*val]; !ok {
						res.list = append(res.list, val)
						res.hashMap[*val] = true
					}
				}
			}
			return
		}
		if val == nil {
			dfs(idx+1, &Value{Index: idx, Data: []rune{data[idx]}})
			dfs(idx+1, val)
		} else {
			dfs(idx+1, &Value{Index: idx, Data: append(val.Data, data[idx])})
			dfs(idx+1, val)
		}
	}
	dfs(0, nil)

	return res.list
}
