package PID

import (
	"fmt"
	"reflect"
)

type PidResultDecoder struct {
	PID      int
	Validate func(frame PIDresponse) error
	Decode   func(frame PIDresponse) (string, error)
}

func validateSize(frame PIDresponse, size int, metric string) error {
	if len(frame.data) < size {
		return fmt.Errorf("not enough data for %s, expected %d bytes", metric, size)
	}
	return nil
}

var PidDefaultDecoders = []PidResultDecoder{
	{PID: PID_RPM,
		Validate: func(frame PIDresponse) error {
			return validateSize(frame, 2, "RPM")
		},
		Decode: func(frame PIDresponse) (string, error) {
			rpm := (int(frame.data[0])<<8 | int(frame.data[1])) / 4
			return fmt.Sprintf("%d RPM", rpm), nil
		},
	},
	{PID: PID_SPEED_KPH,
		Validate: func(frame PIDresponse) error {
			return validateSize(frame, 1, "Speed")
		},
		Decode: func(frame PIDresponse) (string, error) {
			speed := int(frame.data[0])
			return fmt.Sprintf("%d KPH", speed), nil
		},
	},
}

// useful for later perhaps?

// func main() {
// 	u := &User{}

// 	_ = SetField(u, "Name", "Alice")
// 	_ = SetField(u, "Age", 30)
// }

func SetField(target any, fieldName string, value any) error {
	// 1. Get the reflect.Value of the pointer, then dereference it with .Elem()
	v := reflect.ValueOf(target)
	if v.Kind() != reflect.Ptr || v.Elem().Kind() != reflect.Struct {
		return fmt.Errorf("target must be a pointer to a struct")
	}
	v = v.Elem()

	// 2. Find the field by name
	field := v.FieldByName(fieldName)
	if !field.IsValid() {
		return fmt.Errorf("no such field: %s", fieldName)
	}

	// 3. Ensure the field is exported and addressable
	if !field.CanSet() {
		return fmt.Errorf("cannot set field %s (must be exported)", fieldName)
	}

	// 4. Verify the incoming value matches the field's type
	val := reflect.ValueOf(value)
	if field.Type() != val.Type() {
		return fmt.Errorf("type mismatch: expected %s, got %s", field.Type(), val.Type())
	}

	// 5. Assign the value
	field.Set(val)
	return nil
}
