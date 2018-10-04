package xml

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"reflect"
	"testing"
	"time"
)

var (
	testData = map[string]interface{}{
		// boolean
		"<boolean>1</boolean>": true,
		// numbers
		"<int>-5</int>":          -5,
		"<double>1.201</double>": 1.2010,
		// string
		"<string>hello</string>":                   "hello",
		"<string>&lt;&gt;&amp;&#34;&#39;</string>": `<>&"'`,
		// empty array
		"<array><data></data></array>": []interface{}{},
		// array
		"<array><data><value><int>1</int></value><value><int>2</int></value></data></array>": []int{1, 2},
		// nested array
		"<array><data><value><array><data><value><int>1</int></value><value><int>2</int></value></data></array></value></data></array>": []interface{}{[]int{1, 2}},
		// base64
		"<base64>aGVsbG8=</base64>": []byte("hello"),
		// datetime
		"<dateTime.iso8601>20040101T12:30:10</dateTime.iso8601>": time.Date(2004, time.January, 1, 12, 30, 10, 0, time.UTC),
		// empty struct
		"<struct></struct>": struct{}{},
		// struct
		"<struct><member><name>firstname</name><value><string>Kofi</string></value></member></struct>": map[string]interface{}{
			"firstname": "Kofi",
		},
		"<struct><member><name>age</name><value><int>10</int></value></member></struct>": struct {
			Number int `rpc:"age"`
		}{Number: 10},
	}

	emptyDataFixtures = []string{
		"",
		"<array><data></data></array>",
		"<struct></struct>",
	}
)

type person struct {
	Name string `rpc:"name"`
	Age  int    `rpc:"age"`
}

func Test_ReadWriteFixtures(t *testing.T) {
	for res, v := range testData {
		valType := reflect.TypeOf(v)
		xval := fmt.Sprintf("<value>%s</value>", res)
		b := bytes.NewBufferString("")
		withCodec(func(c *Codec) error {
			encoded := makeValue(v)

			if err := c.writeRPC(b, v); err != nil {
				assertOk(t, false, err, "encoding error. ", valType)
			}
			assertEqual(t, xval, b.String(), "encoding ", valType)

			decoded := makeValue("")
			if err := c.readRPC(b, &decoded); err != nil {
				assertOk(t, false, "readRPC value with type '", valType, "' ", err)
			}
			assertEqual(t, encoded, decoded, "decoding ", valType)
			return nil
		})
	}
}

func Test_EmptyValues(t *testing.T) {
	withCodec(func(c *Codec) error {
		buf := bytes.NewBufferString("")
		for _, res := range emptyDataFixtures {
			xmlstr := fmt.Sprintf("<value>%s</value>", res)
			buf.WriteString(xmlstr)
			var rpc rpcValue
			c.readRPC(buf, &rpc)
			assertOk(t, rpc.isEmpty(), "empty: ", xmlstr)
		}
		return nil
	})
}

// pipeEncodeDecode encode in and decode result to out
func pipeEncodeDecode(t *testing.T, in interface{}, out interface{}) {
	b := bytes.NewBufferString("")
	withCodec(func(c *Codec) error {
		if err := c.writeRPC(b, in); err != nil {
			assertOk(t, false, err)
		}
		if err := c.readRPC(b, out); err != nil {
			assertOk(t, false, err)
		}
		return nil
	})
}

func Test_ReadwriteValues(t *testing.T) {
	// bool
	var b bool
	pipeEncodeDecode(t, true, &b)
	assertEqual(t, true, b, "bool")

	// string
	var s string
	pipeEncodeDecode(t, "hello", &s)
	assertEqual(t, "hello", s, "string")

	// int
	var n int
	pipeEncodeDecode(t, 20, &n)
	assertEqual(t, 20, n, "int")

	// float
	var d float64
	pipeEncodeDecode(t, 3.142, &d)
	assertEqual(t, 3.142, d, "float")

	// time
	testTime := time.Date(2005, 12, 24, 3, 30, 5, 0, time.UTC)
	var dt time.Time
	pipeEncodeDecode(t, testTime, &dt)
	assertEqual(t, testTime, dt, "time")

	// base64
	b64In := []byte("hello")
	var b64Out []byte
	pipeEncodeDecode(t, b64In, &b64Out)
	assertEqual(t, b64In, b64Out, "base64")

	// slice of ints
	intSliceIn := []int{3, 2}
	var intSliceOut []int
	pipeEncodeDecode(t, intSliceIn, &intSliceOut)
	assertEqual(t, intSliceIn, intSliceOut, "slice of ints")

	// slice of strings
	stringSliceIn := []string{"one", "two"}
	var stringSliceOut []string
	pipeEncodeDecode(t, stringSliceIn, &stringSliceOut)
	assertEqual(t, stringSliceIn, stringSliceOut, "slice of strings")

	// struct
	p1 := person{Name: "Gyedu", Age: 35}
	var p2 person
	pipeEncodeDecode(t, p1, &p2)
	assertEqual(t, p1, p2, "struct")

	// slice of structs
	p3 := []person{person{Name: "Roseline", Age: 35}, person{Name: "Odame", Age: 25}}
	var p4 []person
	pipeEncodeDecode(t, p3, &p4)
	assertEqual(t, p3, p4, "slice of structs")

	// struct with slice of struct
	type friend struct {
		Name    string
		Best    person
		Friends []friend
	}

	p5 := friend{
		Name: "Kofi",
		Best: person{Name: "Richard"},
		Friends: []friend{
			friend{
				Name:    "Roseline",
				Friends: []friend{friend{Name: "Odame"}},
			},
		},
	}
	var p6 friend
	pipeEncodeDecode(t, p5, &p6)
	assertEqual(t, p5, p6, "struct with slice of structs")

	// fault
	f1 := InternalError.New("panic")
	var f2 Fault
	pipeEncodeDecode(t, f1, &f2)
	assertEqual(t, f1, f2, "fault message")
}

func Test_ReadWriteRequest(t *testing.T) {
	b := bytes.NewBufferString("")
	body := person{Name: "Nana", Age: 10}
	withCodec(func(c *Codec) error {
		if err := c.writeRequest(b, "service.Do", body); err != nil {
			assertOk(t, false, "encode request. ", err)
		}
		res := xml.Header + "<methodCall><methodName>service.Do</methodName><params><param>" +
			"<value><struct><member><name>name</name><value><string>Nana</string></value></member>" +
			"<member><name>age</name><value><int>10</int></value></member></struct></value>" +
			"</param></params></methodCall>"
		assertEqual(t, res, b.String(), "encode request")

		var method string
		var p person
		if err := c.readRequest(b, &method, &p); err != nil {
			assertOk(t, false, "decode call. ", err)
		}
		assertEqual(t, "service.Do", method, "decode call method")
		assertEqual(t, body, p, "decode call body")
		return nil
	})
}

func Test_ReadWriteResponse(t *testing.T) {
	b := bytes.NewBufferString("")
	encoded := person{Name: "Nana", Age: 10}
	withCodec(func(c *Codec) error {
		if err := c.writeResponse(b, encoded); err != nil {
			assertOk(t, false, "encoding response. ", err)
		}
		res := xml.Header + "<methodResponse><params><param>" +
			"<value><struct><member><name>name</name><value><string>Nana</string></value></member>" +
			"<member><name>age</name><value><int>10</int></value></member></struct></value>" +
			"</param></params></methodResponse>"
		assertEqual(t, res, b.String(), "encode response")

		var decoded person
		c.readResponse(b, &decoded)
		assertEqual(t, encoded, decoded, "decode response")
		return nil
	})
}

func Test_ReadWriteFault(t *testing.T) {
	b := bytes.NewBufferString("")
	encoded := InternalError.New("error decoding value")
	withCodec(func(c *Codec) error {
		if err := c.writeResponse(b, encoded); err != nil {
			assertOk(t, false, "encode fault. ", err)
		}
		res := xml.Header + "<methodResponse><fault>" +
			"<value><struct><member><name>faultCode</name><value><int>-32603</int></value></member>" +
			"<member><name>faultString</name><value><string>error decoding value</string></value></member></struct></value>" +
			"</fault></methodResponse>"
		assertEqual(t, res, b.String(), "encode fault")

		var params bool
		decoded := c.readResponse(b, &params /*error expected*/)
		assertEqual(t, encoded, decoded, "decode fault")
		return nil
	})
}
