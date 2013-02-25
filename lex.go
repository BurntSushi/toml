package toml

import (
	"fmt"
	"unicode/utf8"
)

type itemType int

const (
	itemError itemType = iota
	itemNIL
	itemEOF
	itemText
	itemString
	itemBool
	itemInteger
	itemFloat
	itemArray // used internally to the lexer
	itemDatetime
	itemKeyGroupStart
	itemKeyGroupEnd
	itemKeyStart
	itemArrayStart
	itemArrayEnd
	itemCommentStart
)

const (
	eof           = 0
	keyGroupStart = '['
	keyGroupEnd   = ']'
	keyGroupSep   = '.'
	keySep        = '='
	arrayStart    = '['
	arrayEnd      = ']'
	arrayValTerm  = ','
	commentStart  = '#'
)

type stateFn func(lx *lexer) stateFn

type lexer struct {
	input string
	start int
	pos   int
	width int
	state stateFn
	items chan item

	arrayDepth int
}

type item struct {
	typ itemType
	val string
}

func (lx *lexer) nextItem() item {
	for {
		select {
		case item := <-lx.items:
			return item
		default:
			lx.state = lx.state(lx)
		}
	}
	panic("not reached")
}

func lex(input string) *lexer {
	lx := &lexer{
		input: input,
		state: lexTop,
		items: make(chan item, 10),
	}
	return lx
}

func (lx *lexer) emit(typ itemType) {
	lx.items <- item{typ, lx.input[lx.start:lx.pos]}
	lx.start = lx.pos
}

func (lx *lexer) next() (r rune) {
	if lx.pos >= len(lx.input) {
		lx.width = 0
		return eof
	}

	r, lx.width = utf8.DecodeRuneInString(lx.input[lx.pos:])
	lx.pos += lx.width
	return r
}

// ignore skips over the pending input before this point.
func (lx *lexer) ignore() {
	lx.start = lx.pos
}

// backup steps back one rune. Can be called only once per call of next.
func (lx *lexer) backup() {
	lx.pos -= lx.width
}

// accept consumes the next rune if it's equal to `valid`.
func (lx *lexer) accept(valid rune) bool {
	if lx.next() == valid {
		return true
	}
	lx.backup()
	return false
}

// peek returns but does not consume the next rune in the input.
func (lx *lexer) peek() rune {
	r := lx.next()
	lx.backup()
	return r
}

// isValTerm returns true if the given character is a value terminator.
// Value terminators depend on whether we're parsing an array.
func (lx *lexer) isValTerm(r rune) bool {
	if lx.arrayDepth == 0 {
		return isWhitespace(r) || isNL(r)
	}
	return isWhitespace(r) || isNL(r) || r == arrayEnd || r == arrayValTerm
}

func (lx *lexer) errorf(format string, v ...interface{}) stateFn {
	lx.items <- item{
		itemError,
		fmt.Sprintf(format, v...),
	}
	return nil
}

// lexTop parses any valid top-level declaration.
// In TOML, everything except for values and comments are always at the
// top level.
func lexTop(lx *lexer) stateFn {
	r := lx.next()
	if r == eof {
		lx.emit(itemEOF)
		return nil
	}
	if isWhitespace(r) || isNL(r) {
		return lexSkip(lx, lexTop)
	}

	switch r {
	case commentStart:
		lx.backup()
		return lexNewLine(lx, lexTop)
	case keyGroupStart:
		lx.emit(itemKeyGroupStart)
		return lexKeyGroupTextStart
	}

	// All top-level declarations are comments, key groups or key-value
	// pairs. We must now expect a key-value pair.
	lx.backup()
	lx.emit(itemKeyStart)
	return lexKey
}

// lexKey slurps up a key name until the first non-whitespace character.
func lexKey(lx *lexer) stateFn {
	r := lx.next()
	if isNL(r) { // XXX: Not part of the spec?
		return lx.errorf("Key names cannot contain new lines.")
	}

	if isWhitespace(r) {
		lx.backup()
		lx.emit(itemText)
		return lexKeySep
	}
	return lexKey
}

// lexKeySep slurps up whitespace up until the key separator '='.
// Assumes that at least one whitespace character was seen after the key name.
// (But not necessarily consumed.)
func lexKeySep(lx *lexer) stateFn {
	r := lx.next()

	if isWhitespace(r) {
		lx.ignore()
		return lexKeySep
	}
	if r == keySep {
		return lexValueStart
	}
	return lx.errorf("Expected key separator '%c' but found '%c'.",
		keySep, r)
}

func lexValueStart(lx *lexer) stateFn {
	if isWhitespace(lx.next()) {
		return lexSkip(lx, lexValueStart)
	}
	lx.backup()
	return lexValue
}

func lexValue(lx *lexer) stateFn {
	lx.ignore()
	r := lx.next()
	if isWhitespace(r) {
		return lexSkip(lx, lexValue)
	}

	switch {
	case r == '\r':
		fallthrough
	case r == '\n':
		return lx.errorf("Expected TOML value, but found nil instead.")
	case r == '"': // strings
		lx.ignore()
		return lexString
	case r == 't': // bool true
		return lexTr
	case r == 'f': // bool false
		return lexFa
	case r == '-': // negative number
		return lexNegative
	case r >= '0' && r <= '9': // any number or date
		return lexNumber
	case r == '.': // special case error message
		return lx.errorf("TOML float values must be of the form '0.x'.")
	case r == arrayStart:
		lx.emit(itemArrayStart)
		return lexArrayStart
	}
	return lx.errorf("Expected TOML value but found '%c' instead.", r)
}

// lexArrayStart consumes an array, assuming that '[' has just been consumed.
func lexArrayStart(lx *lexer) stateFn {
	r := lx.next()
	if isWhitespace(r) || isNL(r) {
		return lexSkip(lx, lexArrayStart)
	}
	lx.arrayDepth++

	// Handle empty arrays.
	if r == arrayEnd {
		return lexArrayEnd
	}

	// look for any value.
	lx.backup()
	return lexCommentOrVal
}

// lexArrayEnd finishes an array. Assumes that ']' has just been consumed.
func lexArrayEnd(lx *lexer) stateFn {
	lx.backup()
	lx.ignore()
	lx.accept(arrayEnd)

	lx.arrayDepth--
	lx.emit(itemArrayEnd)
	return lexValTerm
}

// lexNegative consumes a negative number (could be float or int).
func lexNegative(lx *lexer) stateFn {
	r := lx.next()
	if r == '.' {
		return lx.errorf("TOML float values must be of the form '-0.x'.")
	}
	if r >= '0' && r <= '9' {
		return lexNumber
	}
	return lx.errorf("Expected a digit after negative sign, but found '%c'.", r)
}

// lexNumber consumes a number. It will consume an entire integer, or
// diverge to a float state if a '.' is found. Or it will diverge to a date
// state if a '-' is found.
// It is assumed that the first digit has already been consumed.
func lexNumber(lx *lexer) stateFn {
	r := lx.next()
	if lx.isValTerm(r) {
		lx.backup()
		lx.emit(itemInteger)
		return lexValTerm
	}

	switch {
	case r >= '0' && r <= '9':
		return lexNumber
	case r == '.':
		return lexFloatFirstAfterDot
	case r == '-':
		if lx.pos-lx.start != 5 {
			return lx.errorf("All ISO8601 dates must be in full Zulu form.")
		}
		return lexZuluDatetimeAfterYear
	}
	return lx.errorf("Expected either a digit or a decimal point, but "+
		"found '%c' instead.", r)
}

// lexZuluDatetimeAfterYear consumes the rest of an ISO8601 datetime in
// full Zulu form. Assumes that "YYYY-" has already been consumed.
func lexZuluDatetimeAfterYear(lx *lexer) stateFn {
	formats := []rune{
		// digits are '0'.
		// everything else is direct equality.
		'0', '0', '-', '0', '0',
		'T',
		'0', '0', ':', '0', '0', ':', '0', '0',
		'Z',
	}
	for _, f := range formats {
		r := lx.next()
		if f == '0' {
			if r < '0' || r > '9' {
				return lx.errorf("Expected digit in ISO8601 datetime, "+
					"but found '%c' instead.", r)
			}
		} else if f != r {
			return lx.errorf("Expected '%c' in ISO8601 datetime, "+
				"but found '%c' instead.", f, r)
		}
	}
	lx.emit(itemDatetime)
	return lexValTerm
}

// lexFloatFirstAfterDot starts the consumption of a floating pointer number
// starting with the first digit after the '.'. Namely, there MUST be digit.
func lexFloatFirstAfterDot(lx *lexer) stateFn {
	r := lx.next()
	if r >= '0' && r <= '9' {
		return lexFloat
	}
	if isNL(r) {
		return lx.errorf("Expected a digit after the decimal point, " +
			"but found a new line instead.")
	}
	return lx.errorf("Expected a digit after the decimal point, but "+
		"found '%c' instead.", r)
}

// lexFloat consumes numbers after the decimal point.
// Assuming the first such number has already been consumed.
func lexFloat(lx *lexer) stateFn {
	r := lx.next()
	if lx.isValTerm(r) {
		lx.backup()
		lx.emit(itemFloat)
		return lexValTerm
	}
	if r >= '0' && r <= '9' {
		return lexFloat
	}
	return lx.errorf("Expected a digit but found '%c' instead.", r)
}

// lexString consumes text inside "...". Assumes that the first '"' has
// already been consumed (and ignored).
func lexString(lx *lexer) stateFn {
	switch lx.next() {
	case eof:
		return lx.errorf("Missing closing '\"' for string.")
	case '\r':
		fallthrough
	case '\n':
		return lx.errorf("Strings cannot contain unescaped new lines.")
	case '\\':
		return lexStringEsc
	case '"':
		lx.backup()
		lx.emit(itemString)
		lx.accept('"')
		return lexValTerm
	}
	return lexString
}

// lexStringEsc consumes the first character after an escape sequence.
// By the spec, only the following escape sequences are allowed:
// \0, \t, \n, \r, \" and \\.
func lexStringEsc(lx *lexer) stateFn {
	r := lx.next()
	switch r {
	case '0':
		fallthrough
	case 't':
		fallthrough
	case 'n':
		fallthrough
	case 'r':
		fallthrough
	case '"':
		fallthrough
	case '\\':
		return lexString
	}
	return lx.errorf("Invalid escape sequence '\\%c'.", r)
}

func lexTr(lx *lexer) stateFn {
	r := lx.next()
	if r == 'r' {
		return lexTru
	}
	return lx.errorf("Expected 'true' but found 't%c' instead.", r)
}

func lexTru(lx *lexer) stateFn {
	r := lx.next()
	if r == 'u' {
		return lexTrue
	}
	return lx.errorf("Expected 'true' but found 'tr%c' instead.", r)
}

func lexTrue(lx *lexer) stateFn {
	r := lx.next()
	if r == 'e' {
		lx.emit(itemBool)
		return lexValTerm
	}
	return lx.errorf("Expected 'true' but found 'tru%c' instead.", r)
}

func lexFa(lx *lexer) stateFn {
	r := lx.next()
	if r == 'a' {
		return lexFal
	}
	return lx.errorf("Exepcted 'false' but found 'f%c' instead.", r)
}

func lexFal(lx *lexer) stateFn {
	r := lx.next()
	if r == 'l' {
		return lexFals
	}
	return lx.errorf("Exepcted 'false' but found 'fa%c' instead.", r)
}

func lexFals(lx *lexer) stateFn {
	r := lx.next()
	if r == 's' {
		return lexFalse
	}
	return lx.errorf("Exepcted 'false' but found 'fal%c' instead.", r)
}

func lexFalse(lx *lexer) stateFn {
	r := lx.next()
	if r == 'e' {
		lx.emit(itemBool)
		return lexValTerm
	}
	return lx.errorf("Exepcted 'false' but found 'fals%c' instead.", r)
}

// lexKeyGroupTextStart parses the beginning character of "[...]" key groups,
// and any sub-groups inside of the same brackets (separated by '.').
// It makes sure the first character of each sub-group is not ']' or '.', to
// prevent empty group names.
func lexKeyGroupTextStart(lx *lexer) stateFn {
	r := lx.next()
	if r == '.' || r == ']' {
		lx.errorf("Key group names cannot be empty.")
	}
	return lexKeyGroupText(lx)
}

// lexKeyGroupText parses text inside "[...]". Assumes that "[" and the
// first character has been slurped. Stops at first "]".
// TODO: No effort is made to prevent or deny characters other than '.' and
// ']' in key group names. See issue #56.
func lexKeyGroupText(lx *lexer) stateFn {
	r := lx.next()
	switch r {
	case keyGroupSep:
		lx.backup()
		lx.emit(itemText)
		lx.accept(keyGroupSep)
		lx.ignore()
		return lexKeyGroupTextStart
	case keyGroupEnd:
		lx.backup()
		lx.emit(itemText)
		lx.accept(keyGroupEnd)
		lx.emit(itemKeyGroupEnd)
		return lexNewLine(lx, lexTop)
	}
	return lexKeyGroupText(lx)
}

// lexValTerm enforces that a value is properly terminated.
// It cheats by checking if we're in an array.
func lexValTerm(lx *lexer) stateFn {
	if lx.arrayDepth == 0 { // at top level, so just look for a new line
		return lexNewLine(lx, lexTop)
	}

	return lexTermThenVal
}

// lexCommentOrVal tries to consume the first value of an array while
// handling comments.
func lexCommentOrVal(lx *lexer) stateFn {
	r := lx.next()
	if isWhitespace(r) || isNL(r) {
		return lexCommentOrVal
	}

	if r == commentStart {
		lx.backup()
		lx.ignore()
		return lexNewLine(lx, lexCommentOrVal)
	}

	lx.backup()
	return lexValue
}

// lexTermThenVal consumes an array terminator and starts parsing a value.
// We handle comments too.
func lexTermThenVal(lx *lexer) stateFn {
	r := lx.next()
	if isWhitespace(r) || isNL(r) {
		return lexTermThenVal
	}

	switch r {
	case commentStart:
		lx.backup()
		lx.ignore()
		return lexNewLine(lx, lexTermThenVal) // we still need a terminator
	case arrayValTerm:
		// commas are terminators, so now we need a value or a ']'
		return lexValOrArrayEnd
	case arrayEnd:
		return lexArrayEnd
	}
	return lx.errorf("Expected array terminator ('%c' or '%c'), but found "+
		"'%c' instead.", arrayEnd, arrayValTerm, r)
}

// lexValOrArrayEnd looks for ']' and finishes the array if it finds one.
// Otherwise, it looks for a value.
// We handle comments too.
func lexValOrArrayEnd(lx *lexer) stateFn {
	r := lx.next()
	if isWhitespace(r) || isNL(r) {
		return lexValOrArrayEnd
	}

	switch r {
	case commentStart:
		lx.backup()
		lx.ignore()
		return lexNewLine(lx, lexValOrArrayEnd)
	case arrayEnd:
		return lexArrayEnd
	case eof:
		return lx.errorf("Expected array terminator '%c', but got EOF.",
			arrayEnd)
	}
	lx.backup()
	return lexValue
}

// lexNewLine enforces a new line and moves on to nextState.
// Also allows for comment.
func lexNewLine(lx *lexer, nextState stateFn) stateFn {
	r := lx.next()
	if isWhitespace(r) {
		lx.ignore()
		return lexNewLine(lx, nextState)
	}
	switch r {
	case commentStart:
		lx.emit(itemCommentStart)
		return lexComment(lx, nextState)
	case '\r':
		fallthrough
	case '\n':
		lx.accept('\r')
		lx.accept('\n')
		lx.ignore()
		return nextState
	case eof:
		return nil
	}
	return lx.errorf("Expected new line but found '%c' instead.", r)
}

// lexComment slurps up everything until the next line and emits it as
// text for a comment. Assumes that '#' has already been consumed.
func lexComment(lx *lexer, nextState stateFn) stateFn {
	switch lx.next() {
	case '\r':
		fallthrough
	case '\n':
		lx.backup()
		lx.emit(itemText)
		return lexNewLine(lx, nextState)
	case eof:
		lx.emit(itemText)
		lx.emit(itemEOF)
		return nil
	}
	return lexComment(lx, nextState)
}

// lexSkip ignores all slurped input and moves on to the next state.
func lexSkip(lx *lexer, nextState stateFn) stateFn {
	lx.ignore()
	return nextState
}

// isWhitespace returns true if `r` is a whitespace character according
// to the spec.
func isWhitespace(r rune) bool {
	return r == '\t' || r == ' '
}

func isNL(r rune) bool {
	return r == '\n' || r == '\r'
}

func (itype itemType) String() string {
	switch itype {
	case itemError:
		return "Error"
	case itemEOF:
		return "EOF"
	case itemText:
		return "Text"
	case itemString:
		return "String"
	case itemBool:
		return "Bool"
	case itemInteger:
		return "Integer"
	case itemFloat:
		return "Float"
	case itemDatetime:
		return "DateTime"
	case itemKeyGroupStart:
		return "KeyGroupStart"
	case itemKeyGroupEnd:
		return "KeyGroupEnd"
	case itemKeyStart:
		return "KeyStart"
	case itemArrayStart:
		return "ArrayStart"
	case itemArrayEnd:
		return "ArrayEnd"
	case itemCommentStart:
		return "CommentStart"
	}
	panic(fmt.Sprintf("BUG: Unknown type '%s'.", itype))
}

func (item item) String() string {
	return fmt.Sprintf("(%s, %s)", item.typ.String(), item.val)
}
