package toml

import (
	"bytes"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"
)

type parser struct {
	mappings []*mapping
	lx       *lexer
	context  []string // the.current.key.group
}

type mapping struct {
	key   []string
	value interface{}
}

func newMapping() *mapping {
	return &mapping{
		key:   make([]string, 0, 1),
		value: nil,
	}
}

func toMap(ms []*mapping) (map[string]interface{}, error) {
	themap := make(map[string]interface{}, 5)
	implicits := make(map[string]bool)
	getMap := func(key []string) (map[string]interface{}, error) {
		// This is where we make sure that duplicate keys cannot be created.
		// Note that something like:
		//
		//	[x.y.z]
		//	[x]
		//
		// Is allowed, but this is not:
		//
		//	[x]
		//	[x.y.z]
		//	[x]
		//
		// In the former case, `x` is created implicitly by `[x.y.z]` while
		// in the latter, it is created explicitly and therefore should not
		// be allowed to be duplicated.
		var ok bool

		m := themap
		accum := make([]string, 0)
		for _, name := range key[0 : len(key)-1] {
			accum = append(accum, name)
			if _, ok = m[name]; !ok {
				implicits[strings.Join(accum, ".")] = true
				m[name] = make(map[string]interface{}, 5)
			}
			m, ok = m[name].(map[string]interface{})
			if !ok {
				return nil, fmt.Errorf("The key group '%s' is duplicated "+
					"elsewhere as a regular key.", strings.Join(accum, "."))
			}
		}

		// If the last part of the key already exists and wasn't created
		// implicitly, we've got a dupe.
		last := key[len(key)-1]
		implicitKey := strings.Join(append(accum, last), ".")
		if _, ok := m[last]; ok && !implicits[implicitKey] {
			return nil, fmt.Errorf("Key '%s' is a duplicate.", implicitKey)
		}
		return m, nil
	}
	for _, m := range ms {
		submap, err := getMap(m.key)
		if err != nil {
			return nil, err
		}
		base := m.key[len(m.key)-1]

		// At this point, maps have been created explicitly.
		// But if this is just a key group create an empty map and move on.
		if m.value == nil {
			submap[base] = make(map[string]interface{}, 5)
			continue
		}

		// We now expect that `submap[base]` is empty. Otherwise, we've
		// got a duplicate on our hands.
		if _, ok := submap[base]; ok {
			return nil, fmt.Errorf("Key '%s' is a duplicate.",
				strings.Join(m.key, "."))
		}
		submap[base] = m.value
	}

	return themap, nil
}

func parse(data string) (ms map[string]interface{}, err error) {
	defer func() {
		if r := recover(); r != nil {
			err, _ = r.(error)
			return
		}
	}()

	p := &parser{
		mappings: make([]*mapping, 0, 50),
		lx:       lex(data),
	}
	for {
		item := p.next()
		if item.typ == itemEOF {
			break
		}
		p.topLevel(item)
	}

	return toMap(p.mappings)
}

func (p *parser) next() item {
	it := p.lx.nextItem()
	if it.typ == itemError {
		p.errorf("Near line %d: %s", it.line, it.val)
	}
	return it
}

func (p *parser) errorf(format string, v ...interface{}) {
	panic(fmt.Errorf(format, v...))
}

func (p *parser) bug(format string, v ...interface{}) {
	log.Fatalf("BUG: %s\n\n", fmt.Sprintf(format, v...))
}

func (p *parser) expect(typ itemType) item {
	it := p.next()
	p.assertEqual(typ, it.typ)
	return it
}

func (p *parser) assertEqual(expected, got itemType) {
	if expected != got {
		p.bug("Expected '%s' but got '%s'.", expected, got)
	}
}

func (p *parser) topLevel(item item) {
	switch item.typ {
	case itemCommentStart:
		p.expect(itemText)
	case itemKeyGroupStart:
		m := newMapping()
		kg := p.expect(itemText)
		for ; kg.typ == itemText; kg = p.next() {
			m.key = append(m.key, kg.val)
		}
		p.assertEqual(itemKeyGroupEnd, kg.typ)
		p.mappings = append(p.mappings, m)
		p.context = m.key
	case itemKeyStart:
		kname := p.expect(itemText)
		m := newMapping()
		for _, k := range p.context {
			m.key = append(m.key, k)
		}
		m.key = append(m.key, kname.val)
		m.value = p.value(p.next())
		p.mappings = append(p.mappings, m)
	default:
		p.bug("Unexpected type at top level: %s", item.typ)
	}
}

func (p *parser) value(it item) interface{} {
	switch it.typ {
	case itemString:
		return it.val
	case itemBool:
		switch it.val {
		case "true":
			return true
		case "false":
			return false
		}
		p.bug("Expected boolean value, but got '%s'.", it.val)
	case itemInteger:
		num, err := strconv.ParseInt(it.val, 10, 64)
		if err != nil {
			if e, ok := err.(*strconv.NumError); ok &&
				e.Err == strconv.ErrRange {

				p.errorf("Integer '%s' is out of the range of 64-bit "+
					"signed integers.", it.val)
			} else {
				p.bug("Expected integer value, but got '%s'.", it.val)
			}
		}
		return num
	case itemFloat:
		num, err := strconv.ParseFloat(it.val, 64)
		if err != nil {
			if e, ok := err.(*strconv.NumError); ok &&
				e.Err == strconv.ErrRange {

				p.errorf("Float '%s' is out of the range of 64-bit "+
					"IEEE-754 floating-point numbers.", it.val)
			} else {
				p.bug("Expected float value, but got '%s'.", it.val)
			}
		}
		return num
	case itemDatetime:
		t, err := time.Parse("2006-01-02T15:04:05Z", it.val)
		if err != nil {
			p.bug("Expected Zulu formatted DateTime, but got '%s'.", it.val)
		}
		return t
	case itemArrayStart:
		theType := itemNIL
		array := make([]interface{}, 0)
		for it = p.next(); it.typ != itemArrayEnd; it = p.next() {
			if it.typ == itemCommentStart {
				p.expect(itemText)
				continue
			}

			if theType == itemNIL {
				theType = it.typ
				array = append(array, p.value(it))
				continue
			}
			if theType != it.typ {
				p.errorf("Array has values of type '%s' and '%s'.",
					theType, it.typ)
			}
			array = append(array, p.value(it))
		}
		return array
	}
	p.bug("Unexpected value type: %s", it.typ)
	panic("unreachable")
}

type mappingsNice []*mapping

func (ms mappingsNice) String() string {
	buf := new(bytes.Buffer)
	for _, m := range ms {
		fmt.Fprintln(buf, strings.Join(m.key, "."))
		fmt.Fprintln(buf, m.value)
		fmt.Fprintln(buf, strings.Repeat("-", 45))
	}
	return buf.String()
}
