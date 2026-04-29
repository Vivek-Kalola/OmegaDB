package parser

import (
	"OmegaDB/node"
	"bytes"
	"errors"
	"fmt"
)

// Parse parses the raw JSON bytes into a lazily-loaded Node tree.
func Parse(data []byte) (*node.Node, error) {
	p := &parser{data: data, pos: 0}
	p.skipWhitespace()
	if p.pos >= len(data) {
		return nil, errors.New("empty JSON data")
	}

	n, err := p.parseValue()
	if err != nil {
		return nil, err
	}

	p.skipWhitespace()
	if p.pos < len(data) {
		return nil, fmt.Errorf("unexpected characters after JSON value at pos %d", p.pos)
	}

	return n, nil
}

type parser struct {
	data []byte
	pos  int
}

func (p *parser) skipWhitespace() {
	for p.pos < len(p.data) {
		c := p.data[p.pos]
		if c == ' ' || c == '\t' || c == '\n' || c == '\r' {
			p.pos++
		} else {
			break
		}
	}
}

func (p *parser) parseValue() (*node.Node, error) {
	if p.pos >= len(p.data) {
		return nil, errors.New("unexpected end of data")
	}

	c := p.data[p.pos]
	switch c {
	case '{':
		return p.parseObject()
	case '[':
		return p.parseArray()
	case '"':
		return p.parseStringNode()
	case 't', 'f':
		return p.parseBool()
	case 'n':
		return p.parseNull()
	default:
		if (c >= '0' && c <= '9') || c == '-' {
			return p.parseNumber()
		}
		return nil, fmt.Errorf("unexpected character '%c' at pos %d", c, p.pos)
	}
}

func (p *parser) parseObject() (*node.Node, error) {
	start := p.pos
	p.pos++ // skip '{'

	n := node.Get()
	n.Type = node.TypeObject

	var lastChild *node.Node

	for {
		p.skipWhitespace()
		if p.pos >= len(p.data) {
			node.Put(n)
			return nil, errors.New("unexpected end of data in object")
		}

		if p.data[p.pos] == '}' {
			p.pos++ // skip '}'
			n.Raw = p.data[start:p.pos]
			return n, nil
		}

		// parse key
		if p.data[p.pos] != '"' {
			node.Put(n)
			return nil, fmt.Errorf("expected '\"' for object key at pos %d", p.pos)
		}

		keyBytes, err := p.parseStringRaw()
		if err != nil {
			node.Put(n)
			return nil, err
		}

		p.skipWhitespace()
		if p.pos >= len(p.data) || p.data[p.pos] != ':' {
			node.Put(n)
			return nil, fmt.Errorf("expected ':' after object key at pos %d", p.pos)
		}
		p.pos++ // skip ':'

		p.skipWhitespace()

		// parse value
		child, err := p.parseValue()
		if err != nil {
			node.Put(n)
			return nil, err
		}

		// Since our node struct doesn't have a separate node for Key and Value,
		// the child itself will hold the Key string.
		// Exclude quotes from key if desired. For now, keyBytes includes the quotes
		// to avoid allocation, or we can just slice inside. Let's slice inside.
		if len(keyBytes) >= 2 {
			child.Key = keyBytes[1 : len(keyBytes)-1] // strip quotes
		} else {
			child.Key = keyBytes
		}

		// append to children list
		if n.Child == nil {
			n.Child = child
		} else {
			lastChild.Next = child
		}
		lastChild = child

		p.skipWhitespace()
		if p.pos >= len(p.data) {
			node.Put(n)
			return nil, errors.New("unexpected end of data in object")
		}

		if p.data[p.pos] == ',' {
			p.pos++
		} else if p.data[p.pos] != '}' {
			node.Put(n)
			return nil, fmt.Errorf("expected ',' or '}' in object at pos %d", p.pos)
		}
	}
}

func (p *parser) parseArray() (*node.Node, error) {
	start := p.pos
	p.pos++ // skip '['

	n := node.Get()
	n.Type = node.TypeArray

	var lastChild *node.Node

	for {
		p.skipWhitespace()
		if p.pos >= len(p.data) {
			node.Put(n)
			return nil, errors.New("unexpected end of data in array")
		}

		if p.data[p.pos] == ']' {
			p.pos++ // skip ']'
			n.Raw = p.data[start:p.pos]
			return n, nil
		}

		// parse element
		child, err := p.parseValue()
		if err != nil {
			node.Put(n)
			return nil, err
		}

		// append to children list
		if n.Child == nil {
			n.Child = child
		} else {
			lastChild.Next = child
		}
		lastChild = child

		p.skipWhitespace()
		if p.pos >= len(p.data) {
			node.Put(n)
			return nil, errors.New("unexpected end of data in array")
		}

		if p.data[p.pos] == ',' {
			p.pos++
		} else if p.data[p.pos] != ']' {
			node.Put(n)
			return nil, fmt.Errorf("expected ',' or ']' in array at pos %d", p.pos)
		}
	}
}

// parseStringRaw returns the raw []byte including quotes.
func (p *parser) parseStringRaw() ([]byte, error) {
	start := p.pos
	p.pos++ // skip opening quote

	for p.pos < len(p.data) {
		c := p.data[p.pos]
		if c == '"' {
			// check for escape
			escaped := false
			for i := p.pos - 1; i >= start+1 && p.data[i] == '\\'; i-- {
				escaped = !escaped
			}
			if !escaped {
				p.pos++ // skip closing quote
				return p.data[start:p.pos], nil
			}
		}
		p.pos++
	}
	return nil, errors.New("unexpected end of data in string")
}

func (p *parser) parseStringNode() (*node.Node, error) {
	raw, err := p.parseStringRaw()
	if err != nil {
		return nil, err
	}
	n := node.Get()
	n.Type = node.TypeString
	n.Raw = raw
	return n, nil
}

func (p *parser) parseNumber() (*node.Node, error) {
	start := p.pos
	for p.pos < len(p.data) {
		c := p.data[p.pos]
		if (c >= '0' && c <= '9') || c == '-' || c == '+' || c == '.' || c == 'e' || c == 'E' {
			p.pos++
		} else {
			break
		}
	}
	if start == p.pos {
		return nil, errors.New("invalid number")
	}

	n := node.Get()
	n.Type = node.TypeNumber
	n.Raw = p.data[start:p.pos]
	return n, nil
}

func (p *parser) parseBool() (*node.Node, error) {
	start := p.pos
	if bytes.HasPrefix(p.data[p.pos:], []byte("true")) {
		p.pos += 4
		n := node.Get()
		n.Type = node.TypeBool
		n.Raw = p.data[start:p.pos]
		return n, nil
	} else if bytes.HasPrefix(p.data[p.pos:], []byte("false")) {
		p.pos += 5
		n := node.Get()
		n.Type = node.TypeBool
		n.Raw = p.data[start:p.pos]
		return n, nil
	}
	return nil, fmt.Errorf("invalid boolean at pos %d", p.pos)
}

func (p *parser) parseNull() (*node.Node, error) {
	start := p.pos
	if bytes.HasPrefix(p.data[p.pos:], []byte("null")) {
		p.pos += 4
		n := node.Get()
		n.Type = node.TypeNull
		n.Raw = p.data[start:p.pos]
		return n, nil
	}
	return nil, fmt.Errorf("invalid null at pos %d", p.pos)
}
