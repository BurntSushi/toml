//! Parser — ported from parse.go (846 LOC)
//!
//! Consumes tokens from the lexer and produces a `Value` tree.

use crate::lex::{Token, TokenWithPos};
use crate::error::ParseError;
use crate::Value;
use std::collections::BTreeMap;

/// Parse a token stream into a TOML `Value` tree.
pub fn parse(tokens: Vec<TokenWithPos>) -> Result<Value, ParseError> {
    let mut parser = Parser::new(tokens);
    parser.parse_document()
}

struct Parser {
    tokens: Vec<TokenWithPos>,
    pos: usize,
}

impl Parser {
    fn new(tokens: Vec<TokenWithPos>) -> Self {
        Parser { tokens, pos: 0 }
    }

    fn current(&self) -> &Token {
        &self.tokens[self.pos].token
    }

    fn current_pos(&self) -> (usize, usize) {
        (self.tokens[self.pos].line, self.tokens[self.pos].col)
    }

    fn advance(&mut self) -> Token {
        let tok = self.tokens[self.pos].token.clone();
        if self.pos < self.tokens.len() - 1 {
            self.pos += 1;
        }
        tok
    }

    fn skip_whitespace_and_comments(&mut self) {
        while self.pos < self.tokens.len() {
            match self.current() {
                Token::Whitespace => { self.advance(); }
                Token::Comment(_) => { self.advance(); }
                _ => break,
            }
        }
    }

    fn skip_newlines_and_whitespace(&mut self) {
        while self.pos < self.tokens.len() {
            match self.current() {
                Token::Whitespace => { self.advance(); }
                Token::Newline => { self.advance(); }
                Token::Comment(_) => { self.advance(); }
                _ => break,
            }
        }
    }

    fn parse_document(&mut self) -> Result<Value, ParseError> {
        let mut root = BTreeMap::new();
        let mut current_table: Vec<String> = Vec::new();

        self.skip_newlines_and_whitespace();

        while !matches!(self.current(), Token::Eof) {
            self.skip_newlines_and_whitespace();
            if matches!(self.current(), Token::Eof) {
                break;
            }

            match self.current() {
                Token::LeftBracket => {
                    // Table header: [a.b.c] or [[a.b.c]] (array of tables)
                    self.parse_table_header(&mut current_table)?;
                    self.ensure_table_exists(&mut root, &current_table)?;
                }
                _ => {
                    // Key-value pair
                    self.parse_key_value(&mut root, &current_table)?;
                }
            }

            self.skip_newlines_and_whitespace();
        }

        Ok(Value::Table(root))
    }

    fn parse_table_header(&mut self, current_table: &mut Vec<String>) -> Result<(), ParseError> {
        self.advance(); // consume [

        let is_array = matches!(self.current(), Token::LeftBracket);
        if is_array {
            self.advance(); // consume second [
        }

        let mut path = Vec::new();
        self.skip_whitespace_and_comments();

        loop {
            let key = self.parse_key()?;
            path.push(key);
            self.skip_whitespace_and_comments();

            if matches!(self.current(), Token::Dot) {
                self.advance();
                self.skip_whitespace_and_comments();
            } else {
                break;
            }
        }

        self.skip_whitespace_and_comments();

        if is_array {
            if !matches!(self.current(), Token::RightBracket) {
                return Err(ParseError::ExpectedToken {
                    line: self.current_pos().0,
                    col: self.current_pos().1,
                    expected: "]",
                    got: format!("{:?}", self.current()),
                });
            }
            self.advance(); // first ]
        }

        if !matches!(self.current(), Token::RightBracket) {
            return Err(ParseError::ExpectedToken {
                line: self.current_pos().0,
                col: self.current_pos().1,
                expected: "]",
                got: format!("{:?}", self.current()),
            });
        }
        self.advance(); // ]

        *current_table = path;

        // TODO: handle array of tables ([[...]])
        if is_array {
            // For now, just treat as a regular table
        }

        Ok(())
    }

    fn parse_key(&mut self) -> Result<String, ParseError> {
        let (line, col) = self.current_pos();
        match self.advance() {
            Token::BareKey(s) => Ok(s),
            Token::String(s) => Ok(s),
            other => Err(ParseError::UnexpectedToken {
                line, col,
                expected: "key",
                got: format!("{:?}", other),
            }),
        }
    }

    fn parse_key_value(&mut self, root: &mut BTreeMap<String, Value>, _current_table: &[String]) -> Result<(), ParseError> {
        // Parse key path (may be dotted: a.b.c = value)
        let mut key_path = Vec::new();
        self.skip_whitespace_and_comments();

        loop {
            let key = self.parse_key()?;
            key_path.push(key);
            self.skip_whitespace_and_comments();

            if matches!(self.current(), Token::Dot) {
                self.advance();
                self.skip_whitespace_and_comments();
            } else {
                break;
            }
        }

        // Expect =
        self.skip_whitespace_and_comments();
        if !matches!(self.current(), Token::Equals) {
            return Err(ParseError::ExpectedToken {
                line: self.current_pos().0,
                col: self.current_pos().1,
                expected: "=",
                got: format!("{:?}", self.current()),
            });
        }
        self.advance(); // =

        // Parse value
        self.skip_whitespace_and_comments();
        let value = self.parse_value()?;

        // Insert into root (handling dotted keys)
        insert_dotted(root, &key_path, value);

        Ok(())
    }

    fn parse_value(&mut self) -> Result<Value, ParseError> {
        let (line, col) = self.current_pos();
        match self.advance() {
            Token::String(s) => Ok(Value::String(s)),
            Token::Integer(n) => Ok(Value::Integer(n)),
            Token::Float(f) => Ok(Value::Float(f)),
            Token::Boolean(b) => Ok(Value::Boolean(b)),
            Token::Datetime(s) => {
                // Parse datetime string into Datetime enum
                // For now, store as string — will be properly parsed
                Ok(Value::String(s))
            },
            Token::LeftBracket => self.parse_array(),
            Token::LeftBrace => self.parse_inline_table(),
            other => Err(ParseError::UnexpectedToken {
                line, col,
                expected: "value",
                got: format!("{:?}", other),
            }),
        }
    }

    fn parse_array(&mut self) -> Result<Value, ParseError> {
        let mut array = Vec::new();

        self.skip_newlines_and_whitespace();

        if matches!(self.current(), Token::RightBracket) {
            self.advance();
            return Ok(Value::Array(array));
        }

        loop {
            self.skip_newlines_and_whitespace();
            let value = self.parse_value()?;
            array.push(value);
            self.skip_newlines_and_whitespace();

            match self.current() {
                Token::Comma => {
                    self.advance();
                    self.skip_newlines_and_whitespace();
                }
                Token::RightBracket => {
                    self.advance();
                    return Ok(Value::Array(array));
                }
                _ => {
                    return Err(ParseError::ExpectedToken {
                        line: self.current_pos().0,
                        col: self.current_pos().1,
                        expected: ", or ]",
                        got: format!("{:?}", self.current()),
                    });
                }
            }
        }
    }

    fn parse_inline_table(&mut self) -> Result<Value, ParseError> {
        let mut table = BTreeMap::new();

        self.skip_whitespace_and_comments();

        if matches!(self.current(), Token::RightBrace) {
            self.advance();
            return Ok(Value::Table(table));
        }

        loop {
            self.skip_whitespace_and_comments();

            // Parse key
            let mut key_path = Vec::new();
            loop {
                let key = self.parse_key()?;
                key_path.push(key);
                self.skip_whitespace_and_comments();

                if matches!(self.current(), Token::Dot) {
                    self.advance();
                    self.skip_whitespace_and_comments();
                } else {
                    break;
                }
            }

            // Expect =
            if !matches!(self.current(), Token::Equals) {
                return Err(ParseError::ExpectedToken {
                    line: self.current_pos().0,
                    col: self.current_pos().1,
                    expected: "=",
                    got: format!("{:?}", self.current()),
                });
            }
            self.advance();
            self.skip_whitespace_and_comments();

            let value = self.parse_value()?;
            insert_dotted(&mut table, &key_path, value);

            self.skip_whitespace_and_comments();

            match self.current() {
                Token::Comma => {
                    self.advance();
                }
                Token::RightBrace => {
                    self.advance();
                    return Ok(Value::Table(table));
                }
                _ => {
                    return Err(ParseError::ExpectedToken {
                        line: self.current_pos().0,
                        col: self.current_pos().1,
                        expected: ", or }",
                        got: format!("{:?}", self.current()),
                    });
                }
            }
        }
    }

    fn ensure_table_exists(&self, _root: &mut BTreeMap<String, Value>, _path: &[String]) -> Result<(), ParseError> {
        // TODO: navigate to the table path and create intermediate tables
        Ok(())
    }
}

fn insert_dotted(table: &mut BTreeMap<String, Value>, path: &[String], value: Value) {
    if path.len() == 1 {
        table.insert(path[0].clone(), value);
    } else {
        let key = &path[0];
        let remaining = &path[1..];
        let entry = table.entry(key.clone()).or_insert(Value::Table(BTreeMap::new()));
        if let Value::Table(t) = entry {
            insert_dotted(t, remaining, value);
        }
    }
}