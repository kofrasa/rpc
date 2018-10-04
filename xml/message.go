package xml

import (
	"fmt"
	"reflect"
	"time"
)

type valueKind byte

const (
	nilKind      valueKind = iota
	booleanKind  valueKind = iota
	intKind      valueKind = iota
	doubleKind   valueKind = iota
	dateTimeKind valueKind = iota
	base64Kind   valueKind = iota
	stringKind   valueKind = iota
	arrayKind    valueKind = iota
	structKind   valueKind = iota
)

var (
	// precomputed types
	typeOfValue     = reflect.TypeOf((*reflect.Value)(nil)).Elem()
	typeOfInterface = reflect.TypeOf((*interface{})(nil)).Elem()
)

// XML-RPC request
type methodCall struct {
	Method string
	rpcParams
}

// XML-RPC response
type methodResponse struct {
	rpcParams
	rpcFault
}

// XML-RPC params
type rpcParams struct {
	Params []rpcValue
}

// XML-RPC fault
type rpcFault struct {
	Fault rpcValue
}

// An XML-RPC value
type rpcValue struct {
	value interface{}
	kind  valueKind
}

// XML-RPC struct entry
type rpcEntry struct {
	Name  string
	Value rpcValue
}

// makeCall creates a new method call
func makeCall(method string, params ...interface{}) methodCall {
	var r methodCall
	r.Method = method
	r.Params = makeParams(params...)
	return r
}

// makeResponse create a new response. Response is a fault if argument is error or of type Fault
func makeResponse(value interface{}) methodResponse {
	var r methodResponse
	switch v := value.(type) {
	case Fault:
		r.Fault = makeValue(v)
	case error:
		r.Fault = makeValue(InternalError.New(v.Error()))
	default:
		r.Params = makeParams(v)
	}
	return r
}

// makeParams creates an slice of XML-RPC values
func makeParams(args ...interface{}) []rpcValue {
	if len(args) == 0 {
		return nil
	}
	arr := make([]rpcValue, 0, len(args))
	for _, v := range args {
		arr = append(arr, makeValue(v))
	}
	return arr
}

// makeValue creates a new XML-RPC value from the given user value
func makeValue(value interface{}) rpcValue {
	var r rpcValue

	// empty value
	if value == nil {
		return r
	}

	// dereference in case of pointer values
	refVal := reflect.ValueOf(value)
	if refVal.Kind() == reflect.Ptr {
		refVal = reflect.Indirect(refVal)
		value = refVal.Interface()
	}

	r.value = value
	r.kind = nilKind

	switch value.(type) {
	case bool:
		r.kind = booleanKind
	case int, int64, int32, int16, uint, uint64, uint32, uint16, uint8:
		r.kind = intKind
	case float64, float32:
		r.kind = doubleKind
	case string:
		r.kind = stringKind
	case []byte:
		r.kind = base64Kind
	case time.Time:
		r.kind = dateTimeKind
	default:
		switch refVal.Kind() {
		case reflect.Slice, reflect.Array:
			var array []rpcValue
			r.value = array // assign nil slice
			r.kind = arrayKind

			size := refVal.Len()
			if size == 0 {
				break
			}

			array = make([]rpcValue, 0, size)
			for i := 0; i < size; i++ {
				item := makeValue(refVal.Index(i).Interface())
				array = append(array, item)
			}
			r.value = array
		case reflect.Map:
			var members []rpcEntry
			r.value = members // assign nil slice
			r.kind = structKind

			mapKeys := refVal.MapKeys()
			if len(mapKeys) == 0 {
				break
			}

			members = make([]rpcEntry, 0, len(mapKeys))
			for _, key := range mapKeys {
				entry := rpcEntry{
					Name:  fmt.Sprintf("%s", key.Interface()),
					Value: makeValue(refVal.MapIndex(key).Interface()),
				}
				members = append(members, entry)
			}

			r.value = members
		case reflect.Struct:
			var members []rpcEntry
			r.value = members // assign nil slice
			r.kind = structKind

			nFields := refVal.NumField()
			if nFields == 0 {
				break
			}

			refType := refVal.Type()
			members = make([]rpcEntry, 0, nFields)
			for i := 0; i < nFields; i++ {
				// get the struct field description
				field := refType.Field(i)
				entry := rpcEntry{
					Name:  field.Name,
					Value: makeValue(refVal.FieldByName(field.Name).Interface()),
				}
				// prefer tags if available
				if tagName, ok := field.Tag.Lookup("rpc"); ok {
					entry.Name = tagName
				}
				members = append(members, entry)
			}

			r.value = members
			r.kind = structKind
		}
	}
	return r
}

// writeTo writes the XML-RPC value to the given pointer value
func (r *rpcValue) writeTo(v interface{}) error {

	// nothing to write
	if r == nil || r.isEmpty() {
		return nil
	}

	// properties of pointer value
	refPtrVal := reflect.ValueOf(v)
	refPtrType := reflect.TypeOf(v)
	refPtrKind := refPtrType.Kind()

	if refPtrKind != reflect.Ptr {
		return InternalError.New("error writing value. expected pointer got '%s'", refPtrKind)
	}

	// properties of reference value
	refType := refPtrType.Elem()
	refKind := refType.Kind()
	refVal := refPtrVal.Elem()

	if refKind == reflect.Interface {
		return InternalError.New("error writing value. cannot write to type '%s'", refPtrKind)
	}

	if refType == typeOfValue {
		refVal = reflect.Value(refVal.Interface().(reflect.Value))
		refKind = refVal.Kind()
		refType = refVal.Type()
	}

	if !refVal.CanSet() {
		return InternalError.New("error writing to value. cannot set value")
	}

	var err error
	val := r.value

	switch r.kind {
	case arrayKind:
		if refType == typeOfInterface {
			// we have an array of generic types. nothing sensible can be done at this point
			// expect the user to know how to interpret the values
			break
		}

		if refKind != reflect.Slice {
			return InternalError.New("error writing value. expected type slice got '%s'", refKind)
		}
		// make our slice
		array, ok := r.value.([]rpcValue)
		if !ok {
			return InternalError.New("invalid decoded type for array")
		}

		size := len(array)
		slice := reflect.MakeSlice(refType, size, size)

		// update our data items
		for i, item := range array {
			m := slice.Index(i)
			if err = item.writeTo(&m); err != nil {
				return err
			}
		}
		// append the new slice to the dereferenced slice
		val = reflect.AppendSlice(refVal, slice).Interface()
	case structKind:
		if refKind != reflect.Struct {
			return InternalError.New("error writing struct. expected type struct got '%s'", refKind)
		}

		members, ok := r.value.([]rpcEntry)
		if !ok {
			return InternalError.New("invalid decoded type for struct")
		}

		nfields := refType.NumField()
		nameMap := make(map[string]string, nfields)
		for i := 0; i < nfields; i++ {
			field := refType.Field(i)
			if name, ok := field.Tag.Lookup("rpc"); ok {
				nameMap[name] = field.Name
			} else {
				nameMap[field.Name] = field.Name
			}
		}

		for _, member := range members {
			fieldName := nameMap[member.Name]
			fieldVal := refVal.FieldByName(nameMap[member.Name])

			// field may not exist, report early to avoid panics
			if !fieldVal.IsValid() {
				return InternalError.New("error writing struct. unknown field %s", fieldName)
			}

			if err = member.Value.writeTo(&fieldVal); err != nil {
				return err
			}
		}

		val = refVal.Interface()
	}

	if err != nil {
		if _, ok := err.(Fault); !ok {
			err = InternalError.New("error writing XML-RPC value. %s", err)
		}
		return err
	}

	if val != nil {
		if reflect.TypeOf(val) != refType && refType != typeOfInterface {
			return InternalError.New("type mismatch: %s != %s", reflect.TypeOf(val), refType)
		}
		refVal.Set(reflect.ValueOf(val))
	}

	return nil
}

// writes parameters to the receiver
func (r *rpcParams) writeTo(args interface{}) error {
	if args == nil || r == nil || len(r.Params) == 0 {
		return nil
	}

	val := reflect.ValueOf(args)
	valKind := val.Kind()

	if valKind != reflect.Ptr {
		return InternalError.New("invalid receiver type. expected pointer but got '%s'", valKind)
	}

	// if we have a single value write it
	if len(r.Params) == 1 {
		return r.Params[0].writeTo(args)
	}

	// otherwie, we are decoding multiple params
	sliceVal := val.Elem()
	array := rpcValue{value: r.Params, kind: arrayKind}
	return array.writeTo(&sliceVal)
}

func (r rpcValue) isEmpty() bool {
	switch r.kind {
	case nilKind:
		return true
	case arrayKind:
		v := r.value.([]rpcValue)
		return len(v) == 0
	case structKind:
		v := r.value.([]rpcEntry)
		return len(v) == 0
	default:
		return false
	}
}

func (r rpcFault) isEmpty() bool {
	return r.Fault.kind == structKind
}
