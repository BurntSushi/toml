//! Lexer — ported from lex.go (1248 LOC)
//!
//! Tokenizes TOML input into a stream of tokens.

use crate::error::ParseError;

#[derive(Debug, Clone, PartialEq)]
pub enum Token {
    // Basic tokens
    Eof,
    Newline,
    Whitespace,

    // Punctuation
    LeftBracket,    // [
    RightBracket,   // ]
    LeftParen,      // (
    RightParen,     // )
    LeftBrace,      // {
    RightBrace,     // }
    Comma,
    Dot,
    Equals,         // =
    Colon,
    DoubleColon,    // ::

    // Values
    String(String),
    BareKey(String),
    Integer(i64),
    Float(f64),
    Boolean(bool),
    Datetime(String), // Raw datetime string, parsed later

    // Comments
    Comment(String),
}

#[derive(Debug, Clone, PartialEq)]
pub struct TokenWithPos {
    pub token: Token,
    pub line: usize,
    pub col: usize,
    pub start: usize, // byte offset
    pub end: usize,   // byte offset
}

/// Lex a TOML string into a vector of tokens with positions.
pub fn lex(input: &str) -> Result<Vec<TokenWithPos>, ParseError> {
    let mut tokens = Vec::new();
    let chars: Vec<char> = input.chars().collect();
    let mut pos = 0;
    let mut line = 1;
    let mut col = 1;

    while pos < chars.len() {
        let c = chars[pos];

        match c {
            // Whitespace (space, tab)
            ' ' | '\t' => {
                let start = pos;
                while pos < chars.len() && (chars[pos] == ' ' || chars[pos] == '\t') {
                    pos += 1;
                    col += 1;
                }
                // Skip whitespace tokens — they're not significant for parsing
            }

            // Newline
            '\n' => {
                tokens.push(TokenWithPos {
                    token: Token::Newline,
                    line,
                    col,
                    start: pos,
                    end: pos + 1,
                });
                pos += 1;
                line += 1;
                col = 1;
            }

            '\r' => {
                // \r\n or standalone \r
                pos += 1;
                if pos < chars.len() && chars[pos] == '\n' {
                    pos += 1;
                }
                tokens.push(TokenWithPos {
                    token: Token::Newline,
                    line,
                    col,
                    start: pos - 2,
                    end: pos,
                });
                line += 1;
                col = 1;
            }

            // Comment
            '#' => {
                let start = pos;
                pos += 1;
                let mut content = String::new();
                while pos < chars.len() && chars[pos] != '\n' && chars[pos] != '\r' {
                    content.push(chars[pos]);
                    pos += 1;
                    col += 1;
                }
                tokens.push(TokenWithPos {
                    token: Token::Comment(content.trim().to_string()),
                    line,
                    col,
                    start,
                    end: pos,
                });
            }

            // Left bracket
            '[' => {
                tokens.push(TokenWithPos {
                    token: Token::LeftBracket,
                    line, col, start: pos, end: pos + 1,
                });
                pos += 1;
                col += 1;
            }

            // Right bracket
            ']' => {
                tokens.push(TokenWithPos {
                    token: Token::RightBracket,
                    line, col, start: pos, end: pos + 1,
                });
                pos += 1;
                col += 1;
            }

            // Left brace (inline table)
            '{' => {
                tokens.push(TokenWithPos {
                    token: Token::LeftBrace,
                    line, col, start: pos, end: pos + 1,
                });
                pos += 1;
                col += 1;
            }

            // Right brace
            '}' => {
                tokens.push(TokenWithPos {
                    token: Token::RightBrace,
                    line, col, start: pos, end: pos + 1,
                });
                pos += 1;
                col += 1;
            }

            // Comma
            ',' => {
                tokens.push(TokenWithPos {
                    token: Token::Comma,
                    line, col, start: pos, end: pos + 1,
                });
                pos += 1;
                col += 1;
            }

            // Dot (dotted keys)
            '.' => {
                tokens.push(TokenWithPos {
                    token: Token::Dot,
                    line, col, start: pos, end: pos + 1,
                });
                pos += 1;
                col += 1;
            }

            // Equals
            '=' => {
                tokens.push(TokenWithPos {
                    token: Token::Equals,
                    line, col, start: pos, end: pos + 1,
                });
                pos += 1;
                col += 1;
            }

            // String literals (basic, literal, multi-line)
            '"' => {
                let (tok, new_pos) = lex_string(&chars, pos, line, col)?;
                let len = new_pos - pos;
                tokens.push(TokenWithPos {
                    token: tok,
                    line, col, start: pos, end: new_pos,
                });
                pos = new_pos;
                col += len;
            }

            '\'' => {
                let (tok, new_pos) = lex_literal_string(&chars, pos, line, col)?;
                let len = new_pos - pos;
                tokens.push(TokenWithPos {
                    token: tok,
                    line, col, start: pos, end: new_pos,
                });
                pos = new_pos;
                col += len;
            }

            // Numbers, booleans, bare keys, dates
            _ => {
                let (tok, new_pos) = lex_value(&chars, pos, line, col)?;
                let len = new_pos - pos;
                tokens.push(TokenWithPos {
                    token: tok,
                    line, col, start: pos, end: new_pos,
                });
                pos = new_pos;
                col += len;
            }
        }
    }

    tokens.push(TokenWithPos {
        token: Token::Eof,
        line, col, start: pos, end: pos,
    });

    Ok(tokens)
}

/// Lex a basic/multi-line basic string starting at `"` or `"""`
fn lex_string(chars: &[char], start: usize, line: usize, col: usize) -> Result<(Token, usize), ParseError> {
    // TODO: implement basic string + multi-line basic string lexing
    // For now, a simple basic string:
    let mut pos = start + 1; // skip opening quote
    let mut result = String::new();

    while pos < chars.len() {
        let c = chars[pos];
        if c == '"' {
            return Ok((Token::String(result), pos + 1));
        }
        if c == '\\' {
            // Escape sequence
            pos += 1;
            if pos >= chars.len() {
                return Err(ParseError::UnexpectedEof { line, col });
            }
            let esc = chars[pos];
            match esc {
                'n' => result.push('\n'),
                't' => result.push('\t'),
                'r' => result.push('\r'),
                '"' => result.push('"'),
                '\\' => result.push('\\'),
                'b' => result.push('\u{0008}'),
                'f' => result.push('\u{000C}'),
                'u' => {
                    // Unicode escape \uXXXX
                    if pos + 4 >= chars.len() {
                        return Err(ParseError::UnexpectedEof { line, col });
                    }
                    let hex: String = chars[pos + 1..pos + 5].iter().collect();
                    let code = u32::from_str_radix(&hex, 16)
                        .map_err(|_| ParseError::InvalidEscape { line, col })?;
                    if let Some(ch) = char::from_u32(code) {
                        result.push(ch);
                    }
                    pos += 4;
                }
                'U' => {
                    // Unicode escape \UXXXXXXXX
                    if pos + 8 >= chars.len() {
                        return Err(ParseError::UnexpectedEof { line, col });
                    }
                    let hex: String = chars[pos + 1..pos + 9].iter().collect();
                    let code = u32::from_str_radix(&hex, 16)
                        .map_err(|_| ParseError::InvalidEscape { line, col })?;
                    if let Some(ch) = char::from_u32(code) {
                        result.push(ch);
                    }
                    pos += 8;
                }
                _ => return Err(ParseError::InvalidEscape { line, col }),
            }
            pos += 1;
        } else {
            result.push(c);
            pos += 1;
        }
    }

    Err(ParseError::UnterminatedString { line, col })
}

/// Lex a literal/multi-line literal string starting at `'` or `'''`
fn lex_literal_string(chars: &[char], start: usize, line: usize, col: usize) -> Result<(Token, usize), ParseError> {
    let mut pos = start + 1; // skip opening quote
    let mut result = String::new();

    while pos < chars.len() {
        let c = chars[pos];
        if c == '\'' {
            return Ok((Token::String(result), pos + 1));
        }
        result.push(c);
        pos += 1;
    }

    Err(ParseError::UnterminatedString { line, col })
}

/// Lex a bare key, number, boolean, or date value
fn lex_value(chars: &[char], start: usize, line: usize, col: usize) -> Result<(Token, usize), ParseError> {
    let mut pos = start;
    let mut buf = String::new();

    while pos < chars.len() {
        let c = chars[pos];
        if c == ' ' || c == '\t' || c == '\n' || c == '\r' || c == ',' || c == ']'
            || c == '}' || c == '#' || c == '=' || c == '.' {
            break;
        }
        buf.push(c);
        pos += 1;
    }

    if buf.is_empty() {
        return Err(ParseError::UnexpectedChar { line, col, char: chars[start] });
    }

    // Try to classify the value
    // Boolean
    if buf == "true" {
        return Ok((Token::Boolean(true), pos));
    }
    if buf == "false" {
        return Ok((Token::Boolean(false), pos));
    }

    // Integer
    if let Ok(n) = parse_integer(&buf) {
        return Ok((Token::Integer(n), pos));
    }

    // Float
    if let Ok(f) = buf.parse::<f64>() {
        return Ok((Token::Float(f), pos));
    }

    // Datetime (heuristic: contains 'T' or '-' in date-like pattern)
    if looks_like_datetime(&buf) {
        return Ok((Token::Datetime(buf), pos));
    }

    // Bare key (A-Za-z0-9_-)
    if buf.chars().all(|c| c.is_alphanumeric() || c == '_' || c == '-') {
        return Ok((Token::BareKey(buf), pos));
    }

    // If nothing else matches, treat as bare key
    Ok((Token::BareKey(buf), pos))
}

fn parse_integer(s: &str) -> Result<i64, ()> {
    // Handle underscores in numbers (1_000_000)
    let cleaned: String = s.chars().filter(|c| *c != '_').collect();

    // Hex, octal, binary
    if cleaned.starts_with("0x") {
        i64::from_str_radix(&cleaned[2..], 16).map_err(|_| ())
    } else if cleaned.starts_with("0o") {
        i64::from_str_radix(&cleaned[2..], 8).map_err(|_| ())
    } else if cleaned.starts_with("0b") {
        i64::from_str_radix(&cleaned[2..], 2).map_err(|_| ())
    } else {
        cleaned.parse::<i64>().map_err(|_| ())
    }
}

fn looks_like_datetime(s: &str) -> bool {
    // Simple heuristic: 4 digits + dash = date-like
    // e.g., 2023-01-01, 2023-01-01T12:00:00, 12:00:00
    let bytes = s.as_bytes();
    if bytes.len() >= 8 && bytes[4] == b'-' {
        return true;
    }
    if bytes.len() >= 5 && bytes[2] == b':' {
        return true;
    }
    false
}