package tag

import (
	"log"
	"strconv"
	"time"
)

func Remove(typedJson interface{}) interface{} {
	switch v := typedJson.(type) {
	case map[string]interface{}:
		if len(v) == 2 && in("type", v) && in("value", v) {
			return untag(v)
		}
		m := make(map[string]interface{}, len(v))
		for k, v2 := range v {
			m[k] = Remove(v2)
		}
		return m
	case []interface{}:
		a := make([]interface{}, len(v))
		for i := range v {
			a[i] = Remove(v[i])
		}
		return a
	}
	log.Fatalf("Unrecognized JSON format '%T'.", typedJson)
	panic("unreachable")
}

func in(key string, m map[string]interface{}) bool {
	_, ok := m[key]
	return ok
}

func untag(typed map[string]interface{}) interface{} {
	t := typed["type"].(string)
	v := typed["value"]
	switch t {
	case "string":
		return v.(string)
	case "integer":
		v := v.(string)
		n, err := strconv.Atoi(v)
		if err != nil {
			log.Fatalf("Could not parse '%s' as integer: %s", v, err)
		}
		return n
	case "float":
		v := v.(string)
		f, err := strconv.ParseFloat(v, 64)
		if err != nil {
			log.Fatalf("Could not parse '%s' as float64: %s", v, err)
		}
		return f
	case "datetime":
		v := v.(string)
		t, err := time.Parse("2006-01-02T15:04:05.999999999Z07:00", v)
		if err != nil {
			log.Fatalf("Could not parse '%s' as a datetime: %s", v, err)
		}
		return t
	case "bool":
		v := v.(string)
		switch v {
		case "true":
			return true
		case "false":
			return false
		}
		log.Fatalf("Could not parse '%s' as a boolean.", v)
	}
	log.Fatalf("Unrecognized tag type '%s'.", t)
	panic("unreachable")
}
