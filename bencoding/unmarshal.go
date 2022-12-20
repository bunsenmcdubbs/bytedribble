package bencoding

import (
	"bytes"
	"errors"
	"strconv"
)

func Unmarshal(raw *bytes.Reader) (result any, err error) {
	if raw.Len() == 0 {
		return nil, nil
	}
	b, err := raw.ReadByte()
	if err != nil {
		return nil, err
	}
	if err := raw.UnreadByte(); err != nil {
		return nil, err
	}
	defer func() {
		if err != nil {
			result = nil
		}
	}()
	switch b {
	case 'i':
		return UnmarshalInt(raw)
	case 'l':
		return UnmarshalList(raw)
	case 'd':
		return UnmarshalDict(raw)
	case '0':
		fallthrough
	case '1':
		fallthrough
	case '2':
		fallthrough
	case '3':
		fallthrough
	case '4':
		fallthrough
	case '5':
		fallthrough
	case '6':
		fallthrough
	case '7':
		fallthrough
	case '8':
		fallthrough
	case '9':
		return UnmarshalString(raw)
	default:
		return nil, errors.New("unrecognized prefix")
	}
}

func UnmarshalInt(raw *bytes.Reader) (int, error) {
	b, err := raw.ReadByte()
	if err != nil {
		return 0, err
	}
	if b != 'i' {
		return 0, errors.New("int encoding must begin with 'i'")
	}

	intBytes := make([]byte, 0)
	b, err = raw.ReadByte()
	if err != nil {
		return 0, err
	}
	if (b < '0' || b > '9') && b != '-' {
		return 0, errors.New("int encoding must start with a number or '-'")
	}
	intBytes = append(intBytes, b)

	for b, err = raw.ReadByte(); err == nil; b, err = raw.ReadByte() {
		if b < '0' || b > '9' {
			if b != 'e' {
				return 0, errors.New("int encoding must end with 'e'")
			}
			break
		}
		intBytes = append(intBytes, b)
	}
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(string(intBytes))
}

func UnmarshalList(raw *bytes.Reader) ([]any, error) {
	b, err := raw.ReadByte()
	if err != nil {
		return nil, err
	}
	if b != 'l' {
		return nil, errors.New("list encoding must begin with 'l'")
	}

	var l []any
	for {
		b, err = raw.ReadByte()
		if err != nil {
			return nil, err
		}
		if b == 'e' {
			return l, nil
		}
		if err = raw.UnreadByte(); err != nil {
			return nil, err
		}
		elem, err := Unmarshal(raw)
		if err != nil {
			return nil, err
		}
		l = append(l, elem)
	}
}

func UnmarshalDict(raw *bytes.Reader) (map[string]any, error) {
	return nil, errors.New("not implemented")
}

func UnmarshalString(raw *bytes.Reader) (string, error) {
	lenBytes := make([]byte, 0)
	var b byte
	var err error
	for b, err = raw.ReadByte(); err == nil; b, err = raw.ReadByte() {
		if b == ':' {
			break
		}
		if b < '0' || b > '9' {
			return "", errors.New("string encoding must start with length")
		}
		lenBytes = append(lenBytes, b)
	}
	if err != nil {
		return "", err
	}

	strLen, err := strconv.Atoi(string(lenBytes))
	if err != nil {
		return "", err
	}

	strBytes := make([]byte, strLen, strLen)
	bytesRead, err := raw.Read(strBytes)
	if err != nil {
		return "", err
	}
	if bytesRead != strLen {
		return "", errors.New("declared string length does not match string")
	}
	return string(strBytes), err
}
