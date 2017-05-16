package recovery

import (
	"fmt"
	"reflect"

	"k8s.io/apimachinery/pkg/conversion"
	"k8s.io/apimachinery/pkg/runtime"
)

// decode decodes value of bytes into object.
func decode(decoder runtime.Decoder, value []byte, objPtr runtime.Object) error {
	if _, err := conversion.EnforcePtr(objPtr); err != nil {
		return fmt.Errorf("objPtr must be pointer, got: %T", objPtr)
	}
	_, _, err := decoder.Decode(value, nil, objPtr)
	if err != nil {
		return err
	}
	return nil
}

// decodeList decodes a list of values into a list of objects.
func decodeList(elems [][]byte, listPtr interface{}, decoder runtime.Decoder) error {
	v, err := conversion.EnforcePtr(listPtr)
	if err != nil || v.Kind() != reflect.Slice {
		return fmt.Errorf("listPtr must be pointer to slice, got: %T", listPtr)
	}
	for _, elem := range elems {
		obj, _, err := decoder.Decode(elem, nil, reflect.New(v.Type().Elem()).Interface().(runtime.Object))
		if err != nil {
			return err
		}
		v.Set(reflect.Append(v, reflect.ValueOf(obj).Elem()))
	}
	return nil
}
