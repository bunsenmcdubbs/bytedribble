package bencoding

import (
	"bytes"
	"reflect"
	"sort"
	"strconv"
)

func Marshal(input any) []byte {
	bs := &bytes.Buffer{}
	marshal(bs, input)
	return bs.Bytes()
}

func marshal(bs *bytes.Buffer, input any) {
	switch in := input.(type) {
	case string:
		marshalString(bs, in)
	case int:
		marshalInt(bs, in)
	case []string:
		marshalList(bs, in)
	case []int:
		marshalList(bs, in)
	case map[string]any:
		marshalDict(bs, in)
	default:
		panic("unable to marshal type: " + reflect.ValueOf(input).Type().Name())
	}
}

func MarshalString(s string) []byte {
	bs := &bytes.Buffer{}
	marshalString(bs, s)
	return bs.Bytes()
}

func marshalString(bs *bytes.Buffer, s string) {
	bs.WriteString(strconv.Itoa(len(s)))
	bs.WriteString(":")
	bs.WriteString(s)
}

func MarshalInt(i int) []byte {
	bs := &bytes.Buffer{}
	marshalInt(bs, i)
	return bs.Bytes()
}

func marshalInt(bs *bytes.Buffer, i int) {
	bs.WriteString("i")
	bs.WriteString(strconv.Itoa(i))
	bs.WriteString("e")
}

func MarshalList[T string | int](l []T) []byte {
	bs := &bytes.Buffer{}
	marshalList(bs, l)
	return bs.Bytes()
}

func marshalList[T string | int](bs *bytes.Buffer, l []T) {
	bs.WriteString("l")
	for _, elem := range l {
		marshal(bs, elem)
	}
	bs.WriteString("e")
}

func MarshalDict(d map[string]any) []byte {
	bs := &bytes.Buffer{}
	marshalDict(bs, d)
	return bs.Bytes()
}

func marshalDict(bs *bytes.Buffer, d map[string]any) {
	var sortedKeys []string
	for k := range d {
		sortedKeys = append(sortedKeys, k)
	}
	sort.Strings(sortedKeys)

	bs.WriteString("d")
	for _, k := range sortedKeys {
		marshalString(bs, k)
		marshal(bs, d[k])
	}
	bs.WriteString("e")
}
