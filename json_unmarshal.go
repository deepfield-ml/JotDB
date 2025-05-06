package jotdb

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// unmarshalJSON deserializes JSON data into the target.
func unmarshalJSON(data []byte, target interface{}) error {
	var parser jsonParser
	parser.init(data)
	v, err := parser.parse()
	if err != nil {
		return err
	}

	switch t := target.(type) {
	case *interface{}:
		*t = v
	case *map[string]interface{}:
		if m, ok := v.(map[string]interface{}); ok {
			*t = m
		} else {
			return fmt.Errorf("expected map, got %T", v)
		}
	case *[]interface{}:
		if a, ok := v.([]interface{}); ok {
			*t = a
		} else {
			return fmt.Errorf("expected array, got %T", v)
		}
	default:
		return fmt.Errorf("unsupported target type: %T", target)
	}
	return nil
}

// jsonParser is a simple JSON parser.
type jsonParser struct {
	data  []byte
	pos   int
	total int
}

func (p *jsonParser) init(data []byte) {
	p.data = data
	p.pos = 0
	p.total = len(data)
}

func (p *jsonParser) parse() (interface{}, error) {
	p.skipWhitespace()
	if p.pos >= p.total {
		return nil, errors.New("empty input")
	}

	switch p.data[p.pos] {
	case '{':
		return p.parseObject()
	case '[':
		return p.parseArray()
	case '"':
		return p.parseString()
	case 't':
		if p.pos+4 <= p.total && string(p.data[p.pos:p.pos+4]) == "true" {
			p.pos += 4
			return true, nil
		}
	case 'f':
		if p.pos+5 <= p.total && string(p.data[p.pos:p.pos+5]) == "false" {
			p.pos += 5
			return false, nil
		}
	case 'n':
		if p.pos+4 <= p.total && string(p.data[p.pos:p.pos+4]) == "null" {
			p.pos += 4
			return nil, nil
		}
	case '-', '0', '1', '2', '3', '4', '5', '6', '7', '8', '9':
		return p.parseNumber()
	}
	return nil, fmt.Errorf("invalid JSON at position %d", p.pos)
}

func (p *jsonParser) skipWhitespace() {
	for p.pos < p.total && (p.data[p.pos] == ' ' || p.data[p.pos] == '\n' || p.data[p.pos] == '\r' || p.data[p.pos] == '\t') {
		p.pos++
	}
}

func (p *jsonParser) parseObject() (map[string]interface{}, error) {
	obj := make(map[string]interface{})
	p.pos++ // Skip '{'
	p.skipWhitespace()

	if p.pos < p.total && p.data[p.pos] == '}' {
		p.pos++
		return obj, nil
	}

	for p.pos < p.total {
		p.skipWhitespace()
		if p.data[p.pos] != '"' {
			return nil, fmt.Errorf("expected string key at position %d", p.pos)
		}
		key, err := p.parseString()
		if err != nil {
			return nil, err
		}
		p.skipWhitespace()
		if p.pos >= p.total || p.data[p.pos] != ':' {
			return nil, fmt.Errorf("expected ':' at position %d", p.pos)
		}
		p.pos++
		p.skipWhitespace()
		value, err := p.parse()
		if err != nil {
			return nil, err
		}
		obj[key] = value
		p.skipWhitespace()
		if p.pos < p.total && p.data[p.pos] == '}' {
			p.pos++
			return obj, nil
		}
		if p.pos >= p.total || p.data[p.pos] != ',' {
			return nil, fmt.Errorf("expected ',' or '}' at position %d", p.pos)
		}
		p.pos++
	}
	return nil, fmt.Errorf("unclosed object at position %d", p.pos)
}

func (p *jsonParser) parseArray() ([]interface{}, error) {
	arr := []interface{}{}
	p.pos++ // Skip '['
	p.skipWhitespace()

	if p.pos < p.total && p.data[p.pos] == ']' {
		p.pos++
		return arr, nil
	}

	for p.pos < p.total {
		value, err := p.parse()
		if err != nil {
			return nil, err
		}
		arr = append(arr, value)
		p.skipWhitespace()
		if p.pos < p.total && p.data[p.pos] == ']' {
			p.pos++
			return arr, nil
		}
		if p.pos >= p.total || p.data[p.pos] != ',' {
			return nil, fmt.Errorf("expected ',' or ']' at position %d", p.pos)
		}
		p.pos++
		p.skipWhitespace()
	}
	return nil, fmt.Errorf("unclosed array at position %d", p.pos)
}

func (p *jsonParser) parseString() (string, error) {
	p.pos++ // Skip opening quote
	var sb strings.Builder
	for p.pos < p.total && p.data[p.pos] != '"' {
		c := p.data[p.pos]
		if c == '\\' {
			p.pos++
			if p.pos >= p.total {
				return "", errors.New("incomplete escape sequence")
			}
			switch p.data[p.pos] {
			case '"', '\\', '/':
				sb.WriteByte(p.data[p.pos])
			case 'b':
				sb.WriteByte('\b')
			case 'f':
				sb.WriteByte('\f')
			case 'n':
			 sb.WriteByte('\n')
			case 'r':
				sb.WriteByte('\r')
			case 't':
				sb.WriteByte('\t')
			case 'u':
				if p.pos+4 >= p.total {
					return "", errors.New("incomplete unicode escape")
				}
				code, err := strconv.ParseUint(string(p.data[p.pos+1:p.pos+5]), 16, 32)
				if err != nil {
					return "", err
				}
				sb.WriteRune(rune(code))
				p.pos += 4
			default:
				return "", fmt.Errorf("invalid escape sequence at position %d", p.pos)
			}
			p.pos++
		} else {
			sb.WriteByte(c)
			p.pos++
		}
	}
	if p.pos >= p.total {
		return "", errors.New("unclosed string")
	}
	p.pos++ // Skip closing quote
	return sb.String(), nil
}

func (p *jsonParser) parseNumber() (interface{}, error) {
	start := p.pos
	for p.pos < p.total {
		c := p.data[p.pos]
		if c != '-' && c != '.' && c != 'e' && c != 'E' && c != '+' && (c < '0' || c > '9') {
			break
		}
		p.pos++
	}
	numStr := string(p.data[start:p.pos])
	if strings.Contains(numStr, ".") || strings.Contains(numStr, "e") || strings.Contains(numStr, "E") {
		f, err := strconv.ParseFloat(numStr, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid number at position %d: %s", start, err)
		}
		return f, nil
	}
	i, err := strconv.ParseInt(numStr, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid integer at position %d: %s", start, err)
	}
	return i, nil
}