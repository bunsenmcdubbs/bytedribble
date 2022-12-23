package bencoding

import (
	"errors"
	"fmt"
	"io"
	"strconv"
)

type ByteReader interface {
	ReadByte() (byte, error)
	UnreadByte() error
	Read([]byte) (int, error)
}

func Unmarshal(raw ByteReader) (result any, err error) {
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

func UnmarshalInt(raw ByteReader) (int, error) {
	b, err := raw.ReadByte()
	if err != nil {
		return 0, err
	}
	if b != 'i' {
		return 0, errors.New("int: encoding must begin with 'i'")
	}

	intBytes := make([]byte, 0)
	b, err = raw.ReadByte()
	if err != nil {
		return 0, err
	}
	if (b < '0' || b > '9') && b != '-' {
		return 0, errors.New("int encoding must start with a number or '-'")
	}
	if b == '0' {
		b, err = raw.ReadByte()
		if err != nil {
			return 0, err
		}
		if b != 'e' {
			return 0, errors.New("int: encoding cannot have leading zeros")
		}
	}
	intBytes = append(intBytes, b)

	for b, err = raw.ReadByte(); err == nil; b, err = raw.ReadByte() {
		if b < '0' || b > '9' {
			if b != 'e' {
				return 0, errors.New("int: encoding must end with 'e'")
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

func UnmarshalList(raw ByteReader) ([]any, error) {
	b, err := raw.ReadByte()
	if err != nil {
		return nil, err
	}
	if b != 'l' {
		return nil, errors.New("list: encoding must begin with 'l'")
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

func UnmarshalDict(raw ByteReader) (map[string]any, error) {
	b, err := raw.ReadByte()
	if err != nil {
		return nil, err
	}
	if b != 'd' {
		return nil, errors.New("dict: encoding must begin with 'd'")
	}

	dict := make(map[string]any)
	for {
		b, err = raw.ReadByte()
		if err != nil {
			return nil, err
		}
		if b == 'e' {
			return dict, nil
		}
		if err := raw.UnreadByte(); err != nil {
			return nil, fmt.Errorf("dict: unable to parse key: %w", err)
		}

		key, err := UnmarshalString(raw)
		if err != nil {
			return nil, err
		}
		value, err := Unmarshal(raw)
		if err != nil {
			return nil, fmt.Errorf("dict: unable to parse value for %q: %w", key, err)
		}
		dict[key] = value
	}
}

func UnmarshalString(raw ByteReader) (string, error) {
	lenBytes := make([]byte, 0)
	var b byte
	var err error
	for b, err = raw.ReadByte(); err == nil; b, err = raw.ReadByte() {
		if b == ':' {
			break
		}
		if b < '0' || b > '9' {
			return "", errors.New("string: encoding must start with length")
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
	bytesRead, err := io.ReadFull(raw, strBytes)
	if err != nil {
		return "", fmt.Errorf("string: unable to read declared string length (wanted %d bytes, read %d bytes): %w", strLen, bytesRead, err)
	}
	return string(strBytes), err
}
