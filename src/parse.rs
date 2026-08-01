//! Parser — ported from parse.go (846 LOC)
//! Consumes tokens from the lexer and produces a `Value` tree.

use crate::lex::{Token, TokenWithPos};
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
            Token::Float(f) => Ok(format_float_key(f)),
            Token::Boolean(b) => Ok(b.to_string()),
            Token::Datetime(s) => Ok(s),
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
        match self.adv() {
            Token::String(s) => Ok(Value::String(s)),
            Token::Integer(n) => Ok(Value::Integer(n)),
            Token::Float(f) => Ok(Value::Float(f)),
            Token::Boolean(b) => Ok(Value::Boolean(b)),
            Token::Datetime(s) => Ok(Value::String(s)),
            Token::BareKey(s) => match s.as_str() {
                "inf" | "+inf" => Ok(Value::Float(f64::INFINITY)),
                "-inf" => Ok(Value::Float(f64::NEG_INFINITY)),
                "nan" | "+nan" | "-nan" => Ok(Value::Float(f64::NAN)),
                _ => {
                    // In value position, a bare key is only valid if it's a
                    // recognized keyword. Anything else is an error.
                    Err(ParseError::InvalidValue{line:l,col:c,message:format!("invalid value: {}", s)})
                }
            },
            Token::LeftBracket => self.parse_array(),
            Token::LeftBrace => self.parse_inline_table(),
            other => Err(ParseError::UnexpectedToken{line:l,col:c,expected:"value",got:format!("{:?}",other)}),
        }
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
        self.skipnl(); // Allow newlines for TOML 1.1
        if matches!(self.cur(), Token::RightBrace) { self.adv(); return Ok(Value::Table(tbl)); }
        loop {
            self.skipnl();
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
            self.skipnl();
            self.skipws();
            match self.cur() {
                Token::Comma => {
                    self.adv();
                    // Handle trailing comma: skip newlines/ws and check for }
                    self.skipnl();
                    self.skipws();
                    if matches!(self.cur(), Token::RightBrace) {
                        self.adv();
                        return Ok(Value::Table(tbl));
                    }
                }
                Token::RightBrace => { self.adv(); return Ok(Value::Table(tbl)); }
                _ => return Err(self.err_expected(", or }")),
            }
        }
    }

    fn err_expected(&self, _exp: &str) -> ParseError {
        let (l, c) = self.curpos();
        ParseError::ExpectedToken{line:l,col:c,expected:"placeholder",got:format!("{:?}", self.cur())}
    }
}

fn format_float_key(f: f64) -> String {
    if f.is_nan() { "nan".to_string() }
    else if f.is_infinite() { if f > 0.0 { "inf".to_string() } else { "-inf".to_string() } }
    else { format!("{}", f) }
}

fn nav_tbl<'a>(root: &'a mut BTreeMap<String, Value>, path: &[String]) -> Result<&'a mut BTreeMap<String, Value>, ParseError> {
    let mut cur = root;
    for (i, key) in path.iter().enumerate() {
        let entry = cur.entry(key.clone()).or_insert(Value::Table(BTreeMap::new()));
        match entry {
            Value::Table(t) => { cur = t; }
            Value::Array(a) => {
                // If this is the last key in the path and the array has elements,
                // navigate into the last element (for [[arr]] + [arr.subtab])
                if i == path.len() - 1 {
                    if let Some(Value::Table(t)) = a.last_mut() {
                        cur = t;
                    } else {
                        return Err(ParseError::DuplicateKey{line:0,col:0,key:key.clone()});
                    }
                } else {
                    // Navigate into the last element of the array
                    if let Some(Value::Table(t)) = a.last_mut() {
                        cur = t;
                    } else {
                        return Err(ParseError::DuplicateKey{line:0,col:0,key:key.clone()});
                    }
                }
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