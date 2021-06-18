package toml

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

type parser struct {
	mapping map[string]interface{}
	types   map[string]tomlType
	lx      *lexer

	// A list of keys in the order that they appear in the TOML data.
	ordered []Key

	// the full key for the current hash in scope
	context Key

	// the base key name for everything except hashes
	currentKey string

	// rough approximation of line number
	approxLine int

	// A map of 'key.group.names' to whether they were created implicitly.
	implicits map[string]bool
}

// ParseError is used when a file can't be parsed: for example invalid integer
// literals, duplicate keys, etc.
type ParseError struct {
	Message string
	Line    int
	LastKey string
}

func (pe ParseError) Error() string {
	return fmt.Sprintf("Near line %d (last key parsed '%s'): %s",
		pe.Line, pe.LastKey, pe.Message)
}

func parse(data string) (p *parser, err error) {
	defer func() {
		if r := recover(); r != nil {
			var ok bool
			if err, ok = r.(ParseError); ok {
				return
			}
			panic(r)
		}
	}()

	// Read over BOM; do this here as the lexer calls utf8.DecodeRuneInString()
	// which mangles stuff.
	if strings.HasPrefix(data, "\xff\xfe") || strings.HasPrefix(data, "\xfe\xff") {
		data = data[2:]
	}

	// Examine first few bytes for NULL bytes; this probably means it's a UTF-16
	// file (second byte in surrogate pair being NULL). Again, do this here to
	// avoid having to deal with UTF-8/16 stuff in the lexer.
	ex := 6
	if len(data) < 6 {
		ex = len(data)
	}
	if strings.ContainsRune(data[:ex], 0) {
		return nil, errors.New("files cannot contain NULL bytes; probably using UTF-16; TOML files must be UTF-8")
	}

	p = &parser{
		mapping:   make(map[string]interface{}),
		types:     make(map[string]tomlType),
		lx:        lex(data),
		ordered:   make([]Key, 0),
		implicits: make(map[string]bool),
	}
	for {
		item := p.next()
		if item.typ == itemEOF {
			break
		}
		p.topLevel(item)
	}

	return p, nil
}

func (p *parser) panicf(format string, v ...interface{}) {
	msg := fmt.Sprintf(format, v...)
	panic(ParseError{
		Message: msg,
		Line:    p.approxLine,
		LastKey: p.current(),
	})
}

func (p *parser) next() item {
	it := p.lx.nextItem()
	//fmt.Printf("ITEM %-18s line %-3d │ %q\n", it.typ, it.line, it.val)
	if it.typ == itemError {
		p.panicf("%s", it.val)
	}
	return it
}

func (p *parser) bug(format string, v ...interface{}) {
	panic(fmt.Sprintf("BUG: "+format+"\n\n", v...))
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
		p.approxLine = item.line
		p.expect(itemText)
	case itemTableStart:
		name := p.next()
		p.approxLine = name.line

		var key Key
		for ; name.typ != itemTableEnd && name.typ != itemEOF; name = p.next() {
			key = append(key, p.keyString(name))
		}
		p.assertEqual(itemTableEnd, name.typ)

		p.establishContext(key, false)
		p.setType("", tomlHash)
		p.ordered = append(p.ordered, key)
	case itemArrayTableStart:
		name := p.next()
		p.approxLine = name.line

		var key Key
		for ; name.typ != itemArrayTableEnd && name.typ != itemEOF; name = p.next() {
			key = append(key, p.keyString(name))
		}
		p.assertEqual(itemArrayTableEnd, name.typ)

		p.establishContext(key, true)
		p.setType("", tomlArrayHash)
		p.ordered = append(p.ordered, key)
	case itemKeyStart:
		name := p.next()
		p.approxLine = name.line

		var key Key
		for ; name.typ != itemKeyEnd && name.typ != itemEOF; name = p.next() {
			key = append(key, p.keyString(name))
		}
		p.assertEqual(itemKeyEnd, name.typ)

		p.currentKey = key[len(key)-1]
		if len(key) > 1 {
			for i := range key[:len(key)-1] {
				app := append(p.context, key[i:i+1]...)
				p.addImplicit(app)
				p.establishContext(app, false)
			}
		}

		val, typ := p.value(p.next())
		p.set(p.currentKey, val, typ)
		p.ordered = append(p.ordered, p.context.add(p.currentKey))

		if len(key) > 1 {
			p.context = p.context[:len(key)-2]
		}

		p.currentKey = ""
	default:
		p.bug("Unexpected type at top level: %s", item.typ)
	}
}

// Gets a string for a key (or part of a key in a table name).
func (p *parser) keyString(it item) string {
	switch it.typ {
	case itemText:
		return it.val
	case itemString, itemMultilineString,
		itemRawString, itemRawMultilineString:
		s, _ := p.value(it)
		return s.(string)
	default:
		p.bug("Unexpected key type: %s", it.typ)
		panic("unreachable")
	}
}

var datetimeRepl = strings.NewReplacer(
	"z", "Z",
	"t", "T",
	" ", "T")

// value translates an expected value from the lexer into a Go value wrapped
// as an empty interface.
func (p *parser) value(it item) (interface{}, tomlType) {
	switch it.typ {
	case itemString:
		return p.replaceEscapes(it.val), p.typeOfPrimitive(it)
	case itemMultilineString:
		trimmed := stripFirstNewline(stripEscapedNewlines(it.val))
		return p.replaceEscapes(trimmed), p.typeOfPrimitive(it)
	case itemRawString:
		return it.val, p.typeOfPrimitive(it)
	case itemRawMultilineString:
		return stripFirstNewline(it.val), p.typeOfPrimitive(it)
	case itemBool:
		switch it.val {
		case "true":
			return true, p.typeOfPrimitive(it)
		case "false":
			return false, p.typeOfPrimitive(it)
		}
		p.bug("Expected boolean value, but got '%s'.", it.val)
	case itemInteger:
		if !numUnderscoresOK(it.val) {
			p.panicf("Invalid integer %q: underscores must be surrounded by digits",
				it.val)
		}
		if numHasLeadingZero(it.val) {
			p.panicf("Invalid integer %q: cannot have leading zeroes", it.val)
		}

		num, err := strconv.ParseInt(it.val, 0, 64)
		if err != nil {
			// Distinguish integer values. Normally, it'd be a bug if the lexer
			// provides an invalid integer, but it's possible that the number is
			// out of range of valid values (which the lexer cannot determine).
			// So mark the former as a bug but the latter as a legitimate user
			// error.
			if e, ok := err.(*strconv.NumError); ok &&
				e.Err == strconv.ErrRange {

				p.panicf("Integer '%s' is out of the range of 64-bit "+
					"signed integers.", it.val)
			} else {
				p.bug("Expected integer value, but got '%s'.", it.val)
			}
		}
		return num, p.typeOfPrimitive(it)
	case itemFloat:
		parts := strings.FieldsFunc(it.val, func(r rune) bool {
			switch r {
			case '.', 'e', 'E':
				return true
			}
			return false
		})
		for _, part := range parts {
			if !numUnderscoresOK(part) {
				p.panicf("Invalid float %q: underscores must be surrounded by digits", it.val)
			}
		}
		if len(parts) > 0 && numHasLeadingZero(parts[0]) {
			p.panicf("Invalid float %q: cannot have leading zeroes", it.val)
		}
		if !numPeriodsOK(it.val) {
			// As a special case, numbers like '123.' or '1.e2',
			// which are valid as far as Go/strconv are concerned,
			// must be rejected because TOML says that a fractional
			// part consists of '.' followed by 1+ digits.
			p.panicf("Invalid float %q: '.' must be followed by one or more digits", it.val)
		}
		val := strings.Replace(it.val, "_", "", -1)
		if val == "+nan" || val == "-nan" { // Go doesn't support this, but TOML spec does.
			val = "nan"
		}
		num, err := strconv.ParseFloat(val, 64)
		if err != nil {
			if e, ok := err.(*strconv.NumError); ok && e.Err == strconv.ErrRange {
				p.panicf("Float '%s' is out of the range of 64-bit IEEE-754 floating-point numbers.", it.val)
			} else {
				p.panicf("Invalid float value: %q", it.val)
			}
		}
		return num, p.typeOfPrimitive(it)
	case itemDatetime:
		it.val = datetimeRepl.Replace(it.val)

		var (
			t   time.Time
			ok  bool
			err error
		)
		for _, format := range []string{
			time.RFC3339Nano,
			"2006-01-02T15:04:05.999999999",
			"2006-01-02",
			"15:04:05.999999999",
		} {
			t, err = time.ParseInLocation(format, it.val, time.Local)
			if err == nil {
				ok = true
				break
			}
		}
		if !ok {
			p.panicf("Invalid TOML Datetime: %q.", it.val)
		}
		return t, p.typeOfPrimitive(it)
	case itemArray:
		array := make([]interface{}, 0)
		types := make([]tomlType, 0)

		for it = p.next(); it.typ != itemArrayEnd; it = p.next() {
			if it.typ == itemCommentStart {
				p.expect(itemText)
				continue
			}

			val, typ := p.value(it)
			array = append(array, val)
			types = append(types, typ)
		}
		return array, p.typeOfArray(types)
	case itemInlineTableStart:
		var (
			hash         = make(map[string]interface{})
			outerContext = p.context
			outerKey     = p.currentKey
		)

		p.context = append(p.context, p.currentKey)
		p.currentKey = ""
		for it := p.next(); it.typ != itemInlineTableEnd; it = p.next() {
			if it.typ == itemCommentStart {
				p.expect(itemText)
				continue
			}

			// retrieve key
			k := p.next()
			_ = p.next() // XXX read KeyEnd; temporary
			p.approxLine = k.line
			kname := p.keyString(k)

			// retrieve value
			p.currentKey = kname
			val, typ := p.value(p.next())
			// make sure we keep metadata up to date
			p.setType(kname, typ)
			p.ordered = append(p.ordered, p.context.add(p.currentKey))
			hash[kname] = val
		}
		p.context = outerContext
		p.currentKey = outerKey
		return hash, tomlHash
	}
	p.bug("Unexpected value type: %s", it.typ)
	panic("unreachable")
}

// numHasLeadingZero checks if this number has leading zeroes, allowing for '0',
// +/- signs, and base prefixes.
func numHasLeadingZero(s string) bool {
	if len(s) > 1 && s[0] == '0' && isDigit(rune(s[1])) { // >1 to allow "0" and isDigit to allow 0x
		return true
	}
	if len(s) > 2 && (s[0] == '-' || s[0] == '+') && s[1] == '0' {
		return true
	}
	return false
}

// numUnderscoresOK checks whether each underscore in s is surrounded by
// characters that are not underscores.
func numUnderscoresOK(s string) bool {
	switch s {
	case "nan", "+nan", "-nan", "inf", "-inf", "+inf":
		return true
	}
	accept := false
	for _, r := range s {
		if r == '_' {
			if !accept {
				return false
			}
		}

		// isHexadecimal is a superset of all the permissable characters
		// surrounding an underscore.
		accept = isHexadecimal(r)
	}
	return accept
}

// numPeriodsOK checks whether every period in s is followed by a digit.
func numPeriodsOK(s string) bool {
	period := false
	for _, r := range s {
		if period && !isDigit(r) {
			return false
		}
		period = r == '.'
	}
	return !period
}

// Set the current context of the parser, where the context is either a hash or
// an array of hashes, depending on the value of the `array` parameter.
//
// Establishing the context also makes sure that the key isn't a duplicate, and
// will create implicit hashes automatically.
func (p *parser) establishContext(key Key, array bool) {
	var ok bool

	// Always start at the top level and drill down for our context.
	hashContext := p.mapping
	keyContext := make(Key, 0)

	// We only need implicit hashes for key[0:-1]
	for _, k := range key[0 : len(key)-1] {
		_, ok = hashContext[k]
		keyContext = append(keyContext, k)

		// No key? Make an implicit hash and move on.
		if !ok {
			p.addImplicit(keyContext)
			hashContext[k] = make(map[string]interface{})
		}

		// If the hash context is actually an array of tables, then set
		// the hash context to the last element in that array.
		//
		// Otherwise, it better be a table, since this MUST be a key group (by
		// virtue of it not being the last element in a key).
		switch t := hashContext[k].(type) {
		case []map[string]interface{}:
			hashContext = t[len(t)-1]
		case map[string]interface{}:
			hashContext = t
		default:
			p.panicf("Key '%s' was already created as a hash.", keyContext)
		}
	}

	p.context = keyContext
	if array {
		// If this is the first element for this array, then allocate a new
		// list of tables for it.
		k := key[len(key)-1]
		if _, ok := hashContext[k]; !ok {
			hashContext[k] = make([]map[string]interface{}, 0, 5)
		}

		// Add a new table. But make sure the key hasn't already been used
		// for something else.
		if hash, ok := hashContext[k].([]map[string]interface{}); ok {
			hashContext[k] = append(hash, make(map[string]interface{}))
		} else {
			p.panicf("Key '%s' was already created and cannot be used as an array.", keyContext)
		}
	} else {
		p.setValue(key[len(key)-1], make(map[string]interface{}))
	}
	p.context = append(p.context, key[len(key)-1])
}

// set calls setValue and setType.
func (p *parser) set(key string, val interface{}, typ tomlType) {
	p.setValue(p.currentKey, val)
	p.setType(p.currentKey, typ)
}

// setValue sets the given key to the given value in the current context.
// It will make sure that the key hasn't already been defined, account for
// implicit key groups.
func (p *parser) setValue(key string, value interface{}) {
	var tmpHash interface{}
	var ok bool

	hash := p.mapping
	keyContext := make(Key, 0)
	for _, k := range p.context {
		keyContext = append(keyContext, k)
		if tmpHash, ok = hash[k]; !ok {
			p.bug("Context for key '%s' has not been established.", keyContext)
		}
		switch t := tmpHash.(type) {
		case []map[string]interface{}:
			// The context is a table of hashes. Pick the most recent table
			// defined as the current hash.
			hash = t[len(t)-1]
		case map[string]interface{}:
			hash = t
		default:
			p.bug("Expected hash to have type 'map[string]interface{}', but "+
				"it has '%T' instead.", tmpHash)
		}
	}
	keyContext = append(keyContext, key)

	if _, ok := hash[key]; ok {
		// Normally redefining keys isn't allowed, but the key could have been
		// defined implicitly and it's allowed to be redefined concretely. (See
		// the `valid/implicit-and-explicit-after.toml` in toml-test)
		//
		// But we have to make sure to stop marking it as an implicit. (So that
		// another redefinition provokes an error.)
		//
		// Note that since it has already been defined (as a hash), we don't
		// want to overwrite it. So our business is done.
		if p.isImplicit(keyContext) {
			p.removeImplicit(keyContext)
			return
		}

		// Otherwise, we have a concrete key trying to override a previous
		// key, which is *always* wrong.
		p.panicf("Key '%s' has already been defined.", keyContext)
	}
	hash[key] = value
}

// setType sets the type of a particular value at a given key.
// It should be called immediately AFTER setValue.
//
// Note that if `key` is empty, then the type given will be applied to the
// current context (which is either a table or an array of tables).
func (p *parser) setType(key string, typ tomlType) {
	keyContext := make(Key, 0, len(p.context)+1)
	for _, k := range p.context {
		keyContext = append(keyContext, k)
	}
	if len(key) > 0 { // allow type setting for hashes
		keyContext = append(keyContext, key)
	}
	p.types[keyContext.String()] = typ
}

// addImplicit sets the given Key as having been created implicitly.
func (p *parser) addImplicit(key Key) {
	p.implicits[key.String()] = true
}

// removeImplicit stops tagging the given key as having been implicitly
// created.
func (p *parser) removeImplicit(key Key) {
	p.implicits[key.String()] = false
}

// isImplicit returns true if the key group pointed to by the key was created
// implicitly.
func (p *parser) isImplicit(key Key) bool {
	return p.implicits[key.String()]
}

// current returns the full key name of the current context.
func (p *parser) current() string {
	if len(p.currentKey) == 0 {
		return p.context.String()
	}
	if len(p.context) == 0 {
		return p.currentKey
	}
	return fmt.Sprintf("%s.%s", p.context, p.currentKey)
}

func stripFirstNewline(s string) string {
	if len(s) > 0 && s[0] == '\n' {
		return s[1:]
	}
	if len(s) > 1 && s[0] == '\r' && s[1] == '\n' {
		return s[2:]
	}
	return s
}

// Remove newlines inside triple-quoted strings if a line ends with "\".
func stripEscapedNewlines(s string) string {
	split := strings.Split(s, "\n")
	if len(split) < 1 {
		return s
	}

	escNL := false // Keep track of the last non-blank line was escaped.
	for i, line := range split {
		line = strings.TrimRight(line, " \t\r")

		if len(line) == 0 || line[len(line)-1] != '\\' {
			split[i] = strings.TrimRight(split[i], "\r")
			if !escNL && i != len(split)-1 {
				split[i] += "\n"
			}
			continue
		}

		escBS := true
		for j := len(line) - 1; j >= 0 && line[j] == '\\'; j-- {
			escBS = !escBS
		}
		if escNL {
			line = strings.TrimLeft(line, " \t\r")
		}
		escNL = !escBS

		if escBS {
			split[i] += "\n"
			continue
		}

		split[i] = line[:len(line)-1] // Remove \
		if len(split)-1 > i {
			split[i+1] = strings.TrimLeft(split[i+1], " \t\r")
		}
	}
	return strings.Join(split, "")
}

func (p *parser) replaceEscapes(str string) string {
	var replaced []rune
	s := []byte(str)
	r := 0
	for r < len(s) {
		if s[r] != '\\' {
			c, size := utf8.DecodeRune(s[r:])
			r += size
			replaced = append(replaced, c)
			continue
		}
		r += 1
		if r >= len(s) {
			p.bug("Escape sequence at end of string.")
			return ""
		}
		switch s[r] {
		default:
			p.bug("Expected valid escape code after \\, but got %q.", s[r])
			return ""
		case 'b':
			replaced = append(replaced, rune(0x0008))
			r += 1
		case 't':
			replaced = append(replaced, rune(0x0009))
			r += 1
		case 'n':
			replaced = append(replaced, rune(0x000A))
			r += 1
		case 'f':
			replaced = append(replaced, rune(0x000C))
			r += 1
		case 'r':
			replaced = append(replaced, rune(0x000D))
			r += 1
		case '"':
			replaced = append(replaced, rune(0x0022))
			r += 1
		case '\\':
			replaced = append(replaced, rune(0x005C))
			r += 1
		case 'u':
			// At this point, we know we have a Unicode escape of the form
			// `uXXXX` at [r, r+5). (Because the lexer guarantees this
			// for us.)
			escaped := p.asciiEscapeToUnicode(s[r+1 : r+5])
			replaced = append(replaced, escaped)
			r += 5
		case 'U':
			// At this point, we know we have a Unicode escape of the form
			// `uXXXX` at [r, r+9). (Because the lexer guarantees this
			// for us.)
			escaped := p.asciiEscapeToUnicode(s[r+1 : r+9])
			replaced = append(replaced, escaped)
			r += 9
		}
	}
	return string(replaced)
}

func (p *parser) asciiEscapeToUnicode(bs []byte) rune {
	s := string(bs)
	hex, err := strconv.ParseUint(strings.ToLower(s), 16, 32)
	if err != nil {
		p.bug("Could not parse '%s' as a hexadecimal number, but the "+
			"lexer claims it's OK: %s", s, err)
	}
	if !utf8.ValidRune(rune(hex)) {
		p.panicf("Escaped character '\\u%s' is not valid UTF-8.", s)
	}
	return rune(hex)
}

func isStringType(ty itemType) bool {
	return ty == itemString || ty == itemMultilineString ||
		ty == itemRawString || ty == itemRawMultilineString
}
