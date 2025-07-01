package assert

import (
	"log"
	"reflect"
)

// IsNil checks if the given error is nil.
func IsNil(err error, v ...any) {
	if err != nil {
		log.Panicf("[ASSERT] %v was not nil. %v", err, v)
	}
}

// Never is a function that should never be called.
func Never(v ...any) {
	log.Panicf("[ASSERT] %v", v)
}

// Assert checks if the condition is true.
func Assert(condition bool, v ...any) {
	if !condition {
		log.Panicf("[ASSERT] %v", v)
	}
}

// IsNotNil checks if the given object is not nil.
func IsNotNil(obj any, v ...any) {
	if obj == nil {
		log.Panicf("[ASSERT] %v was nil. %v", obj, v)
	}

	// Handle interfaces whose value is nil.
	// https://mangatmodi.medium.com/go-check-nil-interface-the-right-way-d142776edef1
	switch reflect.TypeOf(obj).Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Ptr, reflect.UnsafePointer, reflect.Slice:
		if reflect.ValueOf(obj).IsNil() {
			log.Panicf("[ASSERT] value of %v was nil (wrapped in interface). %v", obj, v)
		}
	}
}
