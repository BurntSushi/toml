//! Parser — ported from parse.go (846 LOC)
//! Consumes tokens from the lexer and produces a `Value` tree.

use crate::lex::{Token, TokenWithPos, TokenWithPos as TWP};
use crate::error::ParseError;
use crate::Value;
use std::collections::BTreeMap;

pub fn parse(tokens: Vec<TokenWithPos>) -> Result<Value, ParseError> {
    let mut p = Parser::new(tokens);
    p.parse_document()
}

struct Parser { tokens: Vec<TokenWithPos>, pos: usize }

impl Parser {
    fn new(t: Vec<TokenWithPos>) -> Self { Parser{tokens:t, pos:0} }
    fn cur(&self) -> &Token { &self.tokens[self.pos].token }
    fn curpos(&self) -> (usize,usize) { (self.tokens[self.pos].line, self.tokens[self.pos].col) }
    fn adv(&mut self) -> Token { let t=self.tokens[self.pos].token.clone(); if self.pos<self.tokens.len()-1 {self.pos+=1;} t }
    fn skipws(&mut self) { while self.pos<self.tokens.len(){match self.cur(){Token::Whitespace|Token::Comment(_)=>{self.adv();}_=>{break;}}} }
    fn skipnl(&mut self) { while self.pos<self.tokens.len(){match self.cur(){Token::Whitespace|Token::Comment(_)|Token::Newline=>{self.adv();}_=>{break;}}} }

    fn parse_document(&mut self) -> Result<Value, ParseError> {
        let mut root = BTreeMap::new();
        let mut cur_tbl: Vec<String> = Vec::new();
        self.skipnl();
        while !matches!(self.cur(), Token::Eof) {
            self.skipnl();
            if matches!(self.cur(), Token::Eof) { break; }
            match self.cur() {
                Token::LeftBracket => {
                    let (path, is_arr) = self.parse_table_header()?;
                    if is_arr { insert_aot(&mut root, &path)?; }
                    else { ensure_tbl(&mut root, &path)?; }
                    cur_tbl = path;
                }
                _ => {
                    let (kp, val) = self.parse_kv()?;
                    let tgt = nav_tbl(&mut root, &cur_tbl)?;
                    insert_dotted(tgt, &kp, val)?;
                }
            }
            self.skipnl();
        }
        Ok(Value::Table(root))
    }

    fn parse_table_header(&mut self) -> Result<(Vec<String>, bool), ParseError> {
        self.adv(); // [
        let is_arr = matches!(self.cur(), Token::LeftBracket);
        if is_arr { self.adv(); }
        let mut path = Vec::new();
        self.skipws();
        loop {
            path.push(self.parse_key()?);
            self.skipws();
            if matches!(self.cur(), Token::Dot) { self.adv(); self.skipws(); } else { break; }
        }
        self.skipws();
        if is_arr { if !matches!(self.cur(), Token::RightBracket) { return Err(self.err_expected("]")); } self.adv(); }
        if !matches!(self.cur(), Token::RightBracket) { return Err(self.err_expected("]")); }
        self.adv();
        // After header: expect newline or EOF or comment
        self.skipws();
        match self.cur() {
            Token::Newline | Token::Eof | Token::Comment(_) => {}
            _ => { return Err(self.err_expected("newline")); }
        }
        Ok((path, is_arr))
    }

    fn parse_key(&mut self) -> Result<String, ParseError> {
        let (l, c) = self.curpos();
        match self.adv() {
            Token::BareKey(s) => Ok(s),
            Token::String(s) => Ok(s),
            Token::Integer(n) => Ok(n.to_string()),
            Token::Float(f) => Ok(f.to_string()),
            Token::Boolean(b) => Ok(b.to_string()),
            other => Err(ParseError::UnexpectedToken{line:l,col:c,expected:"key",got:format!("{:?}",other)}),
        }
    }

    fn parse_kv(&mut self) -> Result<(Vec<String>, Value), ParseError> {
        let mut kp = Vec::new();
        self.skipws();
        loop {
            kp.push(self.parse_key()?);
            self.skipws();
            if matches!(self.cur(), Token::Dot) { self.adv(); self.skipws(); } else { break; }
        }
        self.skipws();
        if !matches!(self.cur(), Token::Equals) { return Err(self.err_expected("=")); }
        self.adv();
        self.skipws();
        let v = self.parse_value()?;
        Ok((kp, v))
    }

    fn parse_value(&mut self) -> Result<Value, ParseError> {
        let (l, c) = self.curpos();

            // Check if the next token is a Dot — need to re-lex as combined value (float/datetime)
            if self.pos + 1 < self.tokens.len() {
                if matches!(self.tokens[self.pos+1].token, Token::Dot) {
                    if self.pos + 2 < self.tokens.len() {
                        match &self.tokens[self.pos+2].token {
                            Token::Integer(_) | Token::Float(_) | Token::BareKey(_) => {
                                return self.relex_value();
                            }
                            _ => {}
                        }
                    }
                }
            }

            match self.adv() {
                Token::String(s) => Ok(Value::String(s)),
                Token::Integer(n) => {
                    // Check if next token (after whitespace) is a Datetime (date-time with space separator)
                    // e.g., "1987-07-05 17:45:00Z" lexes as Datetime("1987-07-05") then Datetime("17:45:00Z")
                    self.skipws();
                    if matches!(self.cur(), Token::Datetime(_)) {
                        // This was actually a datetime with space separator, but we already consumed it as Integer
                        // We need to re-lex from the original position
                        // Actually, we already consumed the Integer token. Let's combine.
                        let dt_str = format!("{}", n); // but wait, 1987 would be Integer(1987), not Datetime
                        // Actually the lexer would have lexed "1987-07-05" as Datetime because it starts with 4 digits + dash
                        // So this branch shouldn't be hit. Let me handle the Datetime case below.
                    }
                    Ok(Value::Integer(n))
                }
                Token::Float(f) => Ok(Value::Float(f)),
                Token::Boolean(b) => Ok(Value::Boolean(b)),
                Token::Datetime(s) => {
                    // A datetime value. Check if the next token (after whitespace) is also a Datetime
                    // (space-separated date-time: "1987-07-05 17:45:00Z")
                    let mut combined = s;
                    // Save position to allow backtracking
                    let saved_pos = self.pos;
                    self.skipws();
                    if matches!(self.cur(), Token::Datetime(_)) {
                        // Space-separated datetime
                        if let Token::Datetime(s2) = self.adv() {
                            combined.push(' ');
                            combined.push_str(&s2);
                        }
                    } else {
                        // Not a continuation — restore position
                        self.pos = saved_pos;
                    }
                    Ok(Value::String(combined))
                }
                Token::BareKey(s) => match s.as_str() {
                    "inf" | "+inf" => Ok(Value::Float(f64::INFINITY)),
                    "-inf" => Ok(Value::Float(f64::NEG_INFINITY)),
                    "nan" | "+nan" | "-nan" => Ok(Value::Float(f64::NAN)),
                    _ => Ok(Value::String(s)),
                },
                Token::LeftBracket => self.parse_array(),
                Token::LeftBrace => self.parse_inline_table(),
                other => Err(ParseError::UnexpectedToken{line:l,col:c,expected:"value",got:format!("{:?}",other)}),
        }
    }

    /// Re-lex a value by combining the current token, a dot, and the next token
    /// into a single string, then re-classifying it.
    fn relex_value(&mut self) -> Result<Value, ParseError> {
        // Get the string representation of the current token
        let mut combined = token_to_string(&self.tokens[self.pos].token);
        let start_pos = self.pos;

        // Consume current token
        self.adv(); // current
        self.adv(); // dot — add the dot to combined!
        combined.push('.');

        // Now get the next part
        combined.push_str(&token_to_string(&self.cur()));
        self.adv(); // next

        // Keep consuming if there are more dots followed by values
        while matches!(self.cur(), Token::Dot) {
            combined.push('.');
            self.adv();
            combined.push_str(&token_to_string(&self.cur()));
            self.adv();
        }

        // Classify the combined string
        if let Ok(f) = combined.parse::<f64>() {
            return Ok(Value::Float(f));
        }
        if let Ok(n) = parse_int_combined(&combined) {
            return Ok(Value::Integer(n));
        }
        // It's a datetime or version string
        Ok(Value::String(combined))
    }

    fn parse_array(&mut self) -> Result<Value, ParseError> {
        let mut arr = Vec::new();
        self.skipnl();
        if matches!(self.cur(), Token::RightBracket) { self.adv(); return Ok(Value::Array(arr)); }
        loop {
            self.skipnl();
            if matches!(self.cur(), Token::RightBracket) { self.adv(); return Ok(Value::Array(arr)); }
            arr.push(self.parse_value()?);
            self.skipnl();
            match self.cur() {
                Token::Comma => { self.adv(); self.skipnl(); }
                Token::RightBracket => { self.adv(); return Ok(Value::Array(arr)); }
                _ => return Err(self.err_expected(", or ]")),
            }
        }
    }

    fn parse_inline_table(&mut self) -> Result<Value, ParseError> {
        let mut tbl = BTreeMap::new();
        self.skipws();
        if matches!(self.cur(), Token::RightBrace) { self.adv(); return Ok(Value::Table(tbl)); }
        loop {
            self.skipws();
            let mut kp = Vec::new();
            loop {
                kp.push(self.parse_key()?);
                self.skipws();
                if matches!(self.cur(), Token::Dot) { self.adv(); self.skipws(); } else { break; }
            }
            if !matches!(self.cur(), Token::Equals) { return Err(self.err_expected("=")); }
            self.adv(); self.skipws();
            let v = self.parse_value()?;
            let _ = insert_dotted(&mut tbl, &kp, v);
            self.skipws();
            match self.cur() {
                Token::Comma => { self.adv(); }
                Token::RightBrace => { self.adv(); return Ok(Value::Table(tbl)); }
                _ => return Err(self.err_expected(", or }")),
            }
        }
    }

    fn err_expected(&self, exp: &str) -> ParseError {
        let (l, c) = self.curpos();
        ParseError::ExpectedToken{line:l,col:c,expected:"placeholder",got:format!("{:?}", self.cur())}
    }
}

fn token_to_string(t: &Token) -> String {
    match t {
        Token::String(s) => s.clone(),
        Token::BareKey(s) => s.clone(),
        Token::Integer(n) => n.to_string(),
        Token::Float(f) => f.to_string(),
        Token::Boolean(b) => b.to_string(),
        Token::Datetime(s) => s.clone(),
        _ => String::new(),
    }
}

fn parse_int_combined(s: &str) -> Result<i64, ()> {
    let c: String = s.chars().filter(|c| *c != '_').collect();
    c.parse::<i64>().map_err(|_| ())
}

fn nav_tbl<'a>(root: &'a mut BTreeMap<String, Value>, path: &[String]) -> Result<&'a mut BTreeMap<String, Value>, ParseError> {
    let mut cur = root;
    for key in path {
        let entry = cur.entry(key.clone()).or_insert(Value::Table(BTreeMap::new()));
        match entry {
            Value::Table(t) => { cur = t; }
            Value::Array(a) => {
                if let Some(Value::Table(t)) = a.last_mut() { cur = t; }
                else { return Err(ParseError::DuplicateKey{line:0,col:0,key:key.clone()}); }
            }
            _ => return Err(ParseError::DuplicateKey{line:0,col:0,key:key.clone()}),
        }
    }
    Ok(cur)
}

fn ensure_tbl(root: &mut BTreeMap<String, Value>, path: &[String]) -> Result<(), ParseError> {
    let _ = nav_tbl(root, path)?; Ok(())
}

fn insert_aot(root: &mut BTreeMap<String, Value>, path: &[String]) -> Result<(), ParseError> {
    if path.is_empty() { return Err(ParseError::InvalidValue{line:0,col:0,message:"empty path".into()}); }
    let parent = nav_tbl(root, &path[..path.len()-1])?;
    let key = &path[path.len()-1];
    let entry = parent.entry(key.clone()).or_insert(Value::Array(Vec::new()));
    match entry {
        Value::Array(a) => { a.push(Value::Table(BTreeMap::new())); }
        _ => return Err(ParseError::DuplicateKey{line:0,col:0,key:key.clone()}),
    }
    Ok(())
}

fn insert_dotted(tbl: &mut BTreeMap<String, Value>, path: &[String], val: Value) -> Result<(), ParseError> {
    if path.len()==1 { tbl.insert(path[0].clone(), val); Ok(()) }
    else {
        let key=&path[0]; let rem=&path[1..];
        let entry=tbl.entry(key.clone()).or_insert(Value::Table(BTreeMap::new()));
        if let Value::Table(t)=entry { insert_dotted(t, rem, val) }
        else { Err(ParseError::DuplicateKey{line:0,col:0,key:key.clone()}) }
    }
}