//! Type system — ported from type_fields.go (238 LOC) and type_toml.go (65 LOC)
//! TOML type representations for the toml-test protocol.

use std::fmt;

#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum TomlType {
    Integer,
    Float,
    String,
    Boolean,
    Datetime,
    Array,
    Table,
}

impl fmt::Display for TomlType {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        match self {
            TomlType::Integer => write!(f, "Integer"),
            TomlType::Float => write!(f, "Float"),
            TomlType::String => write!(f, "String"),
            TomlType::Boolean => write!(f, "Bool"),
            TomlType::Datetime => write!(f, "Datetime"),
            TomlType::Array => write!(f, "Array"),
            TomlType::Table => write!(f, "Table"),
        }
    }
}
