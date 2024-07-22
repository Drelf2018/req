package req

import (
	"encoding/json"
	"fmt"
	"io"
)

type Marshaler interface {
	MarshalString() (string, error)
}

func Marshal(i any) (string, error) {
	switch i := i.(type) {
	case json.Marshaler:
		b, err := i.MarshalJSON()
		return string(b), err
	case io.Reader:
		b, err := io.ReadAll(i)
		return string(b), err
	case Marshaler:
		return i.MarshalString()
	case fmt.Stringer:
		return i.String(), nil
	case fmt.GoStringer:
		return i.GoString(), nil
	case string:
		return i, nil
	case bool:
		if i {
			return "true", nil
		} else {
			return "false", nil
		}
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64, float32, float64:
		return fmt.Sprint(i), nil
	default:
		b, err := json.Marshal(i)
		return string(b), err
	}
}
