package toml

// tomlType represents a TOML type.
type tomlType interface {
	tomlType()
	String() string
}

type TomlType = tomlType // XXX

type (
	// Bool represents a TOML boolean.
	Bool struct{}

	// String represents a TOML string.
	String struct {
		Literal   bool // As literal string ('..').
		Multiline bool // As multi-line string ("""..""" or '''..''').
	}

	// Int represents a TOML integer.
	Int struct {
		Base  uint8 // Base 2, 8, 10, 16, or 0 (same as 10).
		Width uint8 // Print leading zeros up to width; ignored for base 10.
	}

	// Float represents a TOML float.
	Float struct {
		Exponent bool // As exponent notation.
	}

	// Datetime represents a TOML datetime.
	Datetime struct {
		Format DatetimeFormat // enum: local, date, time
	}

	// DatetimeFormat controls the format to print a datetime.
	DatetimeFormat uint8

	// Table represents a TOML table.
	Table struct {
		Inline bool // As inline table.
		//Dotted bool
		//Merge  bool
	}

	// Array represents a TOML array.
	Array struct {
		SingleLine bool // Print on single line.
	}

	// ArrayTable represents a TOML array table ([[...]]).
	ArrayTable struct {
		Inline bool // As inline x = [{..}] rather than [[..]]
	}
)

func (d DatetimeFormat) String() string {
	switch d {
	default:
		return "<unknown>"
	case DatetimeFormatFull:
		return "full"
	case DatetimeFormatLocal:
		return "local"
	case DatetimeFormatDate:
		return "date"
	case DatetimeFormatTime:
		return "time"
	}
}

const (
	_                   DatetimeFormat = iota
	DatetimeFormatFull                 // 2021-11-20T15:16:17+01:00
	DatetimeFormatLocal                // 2021-11-20T15:16:17
	DatetimeFormatDate                 // 2021-11-20
	DatetimeFormatTime                 // 15:16:17
)

func (t Bool) tomlType()            {}
func (t String) tomlType()          {}
func (t Int) tomlType()             {}
func (t Float) tomlType()           {}
func (t Datetime) tomlType()        {}
func (t Table) tomlType()           {}
func (t Array) tomlType()           {}
func (t ArrayTable) tomlType()      {}
func (t Bool) String() string       { return "Bool" }
func (t String) String() string     { return "String" }
func (t Int) String() string        { return "Integer" }
func (t Float) String() string      { return "Float" }
func (t Datetime) String() string   { return "Datetime" }
func (t Table) String() string      { return "Table" }
func (t Array) String() string      { return "Array" }
func (t ArrayTable) String() string { return "ArrayTable" }

// meta.types may not be defined for a key, so return a zero value.
func asString(t tomlType) String {
	if t == nil {
		return String{}
	}
	return t.(String)
}
func asInt(t tomlType) Int {
	if t == nil {
		return Int{}
	}
	return t.(Int)
}
func asFloat(t tomlType) Float {
	if t == nil {
		return Float{}
	}
	return t.(Float)
}
func asDatetime(t tomlType) Datetime {
	if t == nil {
		return Datetime{}
	}
	return t.(Datetime)
}
func asTable(t tomlType) Table {
	if t == nil {
		return Table{}
	}
	return t.(Table)
}
func asArray(t tomlType) Array {
	if t == nil {
		return Array{}
	}
	return t.(Array)
}

// typeEqual accepts any two types and returns true if they are equal.
func typeEqual(t1, t2 tomlType) bool {
	if t1 == nil || t2 == nil {
		return false
	}
	return t1.String() == t2.String()
}

func typeIsTable(t tomlType) bool {
	return typeEqual(t, Table{}) || typeEqual(t, ArrayTable{})
}
