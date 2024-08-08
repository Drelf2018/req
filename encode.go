package req

import (
	"encoding/json"
	"strconv"
)

type Marshaler interface {
	MarshalString() (string, error)
}

func Marshal(i any) (string, error) {
	if i == nil {
		return "", nil
	}
	switch i := i.(type) {
	case json.Marshaler:
		b, err := i.MarshalJSON()
		return string(b), err
	case Marshaler:
		return i.MarshalString()
	case []byte:
		return string(i), nil
	case string:
		return i, nil
	case bool:
		if i {
			return "true", nil
		}
		return "false", nil
	case int:
		return strconv.FormatInt(int64(i), 10), nil
	case int8:
		return strconv.FormatInt(int64(i), 10), nil
	case int16:
		return strconv.FormatInt(int64(i), 10), nil
	case int32:
		return strconv.FormatInt(int64(i), 10), nil
	case int64:
		return strconv.FormatInt(i, 10), nil
	case uint:
		return strconv.FormatUint(uint64(i), 10), nil
	case uint8:
		return strconv.FormatUint(uint64(i), 10), nil
	case uint16:
		return strconv.FormatUint(uint64(i), 10), nil
	case uint32:
		return strconv.FormatUint(uint64(i), 10), nil
	case uint64:
		return strconv.FormatUint(i, 10), nil
	case float32:
		return strconv.FormatFloat(float64(i), 'f', -1, 32), nil
	case float64:
		return strconv.FormatFloat(float64(i), 'f', -1, 64), nil
	default:
		b, err := json.Marshal(i)
		return string(b), err
	}
}
