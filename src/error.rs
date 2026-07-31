//! Error types — ported from error.go (327 LOC)
//!
//! Typed errors with position information, replacing Go's string-based errors.

use std::fmt;

#[derive(Debug, Clone)]
pub enum ParseError {
    UnexpectedChar {
        line: usize,
        col: usize,
        char: char,
    },
    UnexpectedToken {
        line: usize,
        col: usize,
        expected: &'static str,
        got: String,
    },
    ExpectedToken {
        line: usize,
        col: usize,
        expected: &'static str,
        got: String,
    },
    UnterminatedString {
        line: usize,
        col: usize,
    },
    InvalidEscape {
        line: usize,
        col: usize,
    },
    UnexpectedEof {
        line: usize,
        col: usize,
    },
    DuplicateKey {
        line: usize,
        col: usize,
        key: String,
    },
    InvalidValue {
        line: usize,
        col: usize,
        message: String,
    },
}

impl fmt::Display for ParseError {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        match self {
            ParseError::UnexpectedChar { line, col, char } => {
                write!(f, "toml: error: unexpected character '{}' at line {}, column {}", char, line, col)
            }
            ParseError::UnexpectedToken { line, col, expected, got } => {
                write!(f, "toml: error: expected {} but got {} at line {}, column {}", expected, got, line, col)
            }
            ParseError::ExpectedToken { line, col, expected, got } => {
                write!(f, "toml: error: expected {} but got {} at line {}, column {}", expected, got, line, col)
            }
            ParseError::UnterminatedString { line, col } => {
                write!(f, "toml: error: unterminated string at line {}, column {}", line, col)
            }
            ParseError::InvalidEscape { line, col } => {
                write!(f, "toml: error: invalid escape sequence at line {}, column {}", line, col)
            }
            ParseError::UnexpectedEof { line, col } => {
                write!(f, "toml: error: unexpected end of file at line {}, column {}", line, col)
            }
            ParseError::DuplicateKey { line, col, key } => {
                write!(f, "toml: error: duplicate key '{}' at line {}, column {}", key, line, col)
            }
            ParseError::InvalidValue { line, col, message } => {
                write!(f, "toml: error: invalid value at line {}, column {}: {}", line, col, message)
            }
        }
    }
}

impl std::error::Error for ParseError {}

#[derive(Debug, Clone)]
pub enum EncodeError {
    InvalidValue(String),
}

impl fmt::Display for EncodeError {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        match self {
            EncodeError::InvalidValue(msg) => write!(f, "toml: encode error: {}", msg),
        }
    }
}

impl std::error::Error for EncodeError {}