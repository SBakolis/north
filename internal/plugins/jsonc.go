package plugins

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"strings"
)

type token struct {
	kind       byte
	start, end int
	text       string
}

type node struct {
	kind       byte
	start, end int
	text       string
	elements   []node
	properties []property
}

type property struct {
	name  string
	value node
}

// ValidateConfig verifies that data is a complete JSON or JSONC object without rewriting it.
func ValidateConfig(data []byte) error {
	_, _, err := parseJSONC(data)
	return err
}

func lexJSONC(data []byte) ([]token, error) {
	var tokens []token
	for i := 0; i < len(data); {
		switch data[i] {
		case ' ', '\t', '\r', '\n':
			i++
		case '/':
			if i+1 >= len(data) {
				return nil, fmt.Errorf("unexpected slash at byte %d", i)
			}
			if data[i+1] == '/' {
				i += 2
				for i < len(data) && data[i] != '\n' {
					i++
				}
			} else if data[i+1] == '*' {
				start := i
				i += 2
				for i+1 < len(data) && !(data[i] == '*' && data[i+1] == '/') {
					i++
				}
				if i+1 >= len(data) {
					return nil, fmt.Errorf("unterminated comment at byte %d", start)
				}
				i += 2
			} else {
				return nil, fmt.Errorf("unexpected slash at byte %d", i)
			}
		case '"':
			start := i
			i++
			for i < len(data) {
				if data[i] == '\\' {
					i += 2
					continue
				}
				if data[i] == '"' {
					i++
					break
				}
				i++
			}
			if i > len(data) || data[i-1] != '"' {
				return nil, fmt.Errorf("unterminated string at byte %d", start)
			}
			var text string
			if err := json.Unmarshal(data[start:i], &text); err != nil {
				return nil, fmt.Errorf("string at byte %d: %w", start, err)
			}
			tokens = append(tokens, token{kind: 's', start: start, end: i, text: text})
		case '{', '}', '[', ']', ':', ',':
			tokens = append(tokens, token{kind: data[i], start: i, end: i + 1})
			i++
		default:
			start := i
			for i < len(data) && !strings.ContainsRune(" \t\r\n{}[],:/", rune(data[i])) {
				i++
			}
			if start == i {
				return nil, fmt.Errorf("unexpected byte %q at byte %d", data[i], i)
			}
			tokens = append(tokens, token{kind: 'v', start: start, end: i, text: string(data[start:i])})
		}
	}
	return tokens, nil
}

type parser struct {
	tokens []token
	at     int
}

func parseJSONC(data []byte) (node, []token, error) {
	tokens, err := lexJSONC(data)
	if err != nil {
		return node{}, nil, err
	}
	p := parser{tokens: tokens}
	root, err := p.value()
	if err != nil {
		return node{}, nil, err
	}
	if p.at != len(tokens) {
		return node{}, nil, fmt.Errorf("unexpected token at byte %d", tokens[p.at].start)
	}
	return root, tokens, nil
}

func (p *parser) value() (node, error) {
	if p.at >= len(p.tokens) {
		return node{}, fmt.Errorf("expected value at end of input")
	}
	t := p.tokens[p.at]
	switch t.kind {
	case 's':
		p.at++
		return node{kind: t.kind, start: t.start, end: t.end, text: t.text}, nil
	case 'v':
		if !json.Valid([]byte(t.text)) {
			return node{}, fmt.Errorf("invalid JSON value %q at byte %d", t.text, t.start)
		}
		p.at++
		return node{kind: t.kind, start: t.start, end: t.end, text: t.text}, nil
	case '[':
		return p.array()
	case '{':
		return p.object()
	default:
		return node{}, fmt.Errorf("expected value at byte %d", t.start)
	}
}

func (p *parser) array() (node, error) {
	start := p.tokens[p.at].start
	p.at++
	n := node{kind: '[', start: start}
	for p.at < len(p.tokens) && p.tokens[p.at].kind != ']' {
		v, err := p.value()
		if err != nil {
			return node{}, err
		}
		n.elements = append(n.elements, v)
		if p.at < len(p.tokens) && p.tokens[p.at].kind == ',' {
			p.at++
			continue
		}
		if p.at >= len(p.tokens) || p.tokens[p.at].kind != ']' {
			return node{}, fmt.Errorf("expected comma or ]")
		}
	}
	if p.at >= len(p.tokens) {
		return node{}, fmt.Errorf("unterminated array")
	}
	n.end = p.tokens[p.at].end
	p.at++
	return n, nil
}

func (p *parser) object() (node, error) {
	start := p.tokens[p.at].start
	p.at++
	n := node{kind: '{', start: start}
	for p.at < len(p.tokens) && p.tokens[p.at].kind != '}' {
		key := p.tokens[p.at]
		if key.kind != 's' {
			return node{}, fmt.Errorf("expected object key at byte %d", key.start)
		}
		p.at++
		if p.at >= len(p.tokens) || p.tokens[p.at].kind != ':' {
			return node{}, fmt.Errorf("expected colon after %q", key.text)
		}
		p.at++
		v, err := p.value()
		if err != nil {
			return node{}, err
		}
		n.properties = append(n.properties, property{name: key.text, value: v})
		if p.at < len(p.tokens) && p.tokens[p.at].kind == ',' {
			p.at++
			continue
		}
		if p.at >= len(p.tokens) || p.tokens[p.at].kind != '}' {
			return node{}, fmt.Errorf("expected comma or }")
		}
	}
	if p.at >= len(p.tokens) {
		return node{}, fmt.Errorf("unterminated object")
	}
	n.end = p.tokens[p.at].end
	p.at++
	return n, nil
}

func pluginArrays(root node) []node {
	if root.kind != '{' {
		return nil
	}
	var arrays []node
	for _, prop := range root.properties {
		if prop.name == "plugin" && prop.value.kind == '[' {
			arrays = append(arrays, prop.value)
		}
	}
	return arrays
}

func fingerprint(data []byte, n node) string {
	return fmt.Sprintf("sha256:%x", sha256.Sum256(data[n.start:n.end]))
}
