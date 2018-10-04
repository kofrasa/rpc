package xml

import (
	"fmt"
	"reflect"
	"testing"
)

func assertOk(tb testing.TB, ok bool, v ...interface{}) {
	if ok {
		tb.Logf("pass: %s", fmt.Sprint(v...))
	} else {
		tb.Fatalf("fail: %s", fmt.Sprint(v...))
	}
}

func assertEqual(tb testing.TB, expected, actual interface{}, v ...interface{}) {
	if reflect.DeepEqual(expected, actual) {
		tb.Logf("pass: %s", fmt.Sprint(v...))
	} else {
		tb.Fatalf("fail: %s \nexpected: %#v \nactual: %#v", fmt.Sprint(v...), expected, actual)
	}
}

func assertNotEqual(tb testing.TB, expected, actual interface{}, v ...interface{}) {
	if !reflect.DeepEqual(expected, actual) {
		tb.Logf("pass: %s", fmt.Sprint(v...))
	} else {
		tb.Fatalf("fail: %s", fmt.Sprint(v...))
	}
}
