package jotdb

import (
	"bytes"
	"fmt"
	"strconv"
	"unicode/utf8"
)

// marshalJSON serializes a Go value to JSON.
func marshalJSON(v interface{}) ([]byte, error) {
	var buf bytes.Buffer
	if err := writeJSON(&buf, v); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// writeJSON writes a Go value as JSON to the buffer.
func writeJSON(buf *bytes.Buffer, v interface{}) error {
	switch val := v.(type) {
	case nil:
		buf.WriteString("null")
	case string:
		escapeString(buf, val)
	case float64:
		buf.WriteString(strconv.FormatFloat(val, 'f', -1, 64))
	case int:
		buf.WriteString(strconv.Itoa(val))
	case bool:
		if val {
			buf.WriteString("true")
		} else {
			buf.WriteString("false")
		}
	case map[string]interface{}:
		buf.WriteByte('{')
		first := true
		for k, v := range val {
			if !first {
				buf.WriteByte(',')
			}
			first = false
			escapeString(buf, k)
			buf.WriteByte(':')
			if err := writeJSON(buf, v); err != nil {
				return err
			}
		}
		buf.WriteByte('}')
	case []interface{}:
		buf.WriteByte('[')
		for i, item := range val {
			if i > 0 {
				buf.WriteByte(',')
			}
			if err := writeJSON(buf, item); err != nil {
				return err
			}
		}
		buf.WriteByte(']')
	default:
		return fmt.Errorf("unsupported type: %T", v)
	}
	return nil
}

// escapeString writes a JSON-escaped string to the buffer.
func escapeString(buf *bytes.Buffer, s string) {
	buf.WriteByte('"')
	for _, r := range s {
		switch r {
		case '"':
			buf.WriteString(`\"`)
		case '\\':
			buf.WriteString(`\\`)
		case '\b':
			buf.WriteString(`\b`)
		case '\f':
			buf.WriteString(`\f`)
		case '\n':
			buf.WriteString(`\n`)
		case '\r':
			buf.WriteString(`\r`)
		case '\t':
			buf.WriteString(`\t`)
		default:
			if r < 32 || r >= utf8.MaxRune || r == 0x2028 || r == 0x2029 {
				fmt.Fprintf(buf, `\u%04x`, r)
			} else {
				buf.WriteRune(r)
			}
		}
	}
	buf.WriteByte('"')
}