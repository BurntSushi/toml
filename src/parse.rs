//! Parser — ported from parse.go (846 LOC)
//! Consumes tokens from the lexer and produces a `Value` tree.

use crate::datetime::parse_datetime;
use crate::error::ParseError;
use crate::lex::{Token, TokenWithPos};
use crate::Value;
use std::collections::{BTreeMap, HashMap};

pub fn parse(tokens: Vec<TokenWithPos>) -> Result<Value, ParseError> {
    let mut p = Parser::new(tokens);
    p.parse_document()
}

/// How a given path came to exist.
///
/// The Go original tracks only `implicit`/explicit on table nodes, which is why
/// it accepts things like `a.b.c = 1` followed by `a.b = 2`. Distinguishing the
/// six ways a path can be created makes every "already defined" rule in the
/// spec expressible as a single lookup.
#[derive(Clone, Copy, PartialEq, Eq, Debug)]
enum Kind {
    /// Defined by a `[table]` header.
    Header,
    /// Brought into being as the parent of a `[a.b]` header.
    Implicit,
    /// Defined by a `[[array]]` header.
    Aot,
    /// A super-table created by a dotted key in a key/value pair.
    Dotted,
    /// An inline table value: `{ … }`. Sealed — nothing may extend it.
    Inline,
    /// A super-table created by a dotted key *inside* an inline table.
    InlineDotted,
    /// Any other value: scalar, array, array of inline tables.
    Value,
}

const SEP: char = '\u{1}';
const IDX: char = '\u{2}';

struct Parser {
    tokens: Vec<TokenWithPos>,
    pos: usize,
    /// Canonical path → how it was defined. Array-of-table elements get an
    /// index baked into the canonical path so siblings don't collide.
    meta: HashMap<String, Kind>,
    /// Canonical path of an array of tables → index of its current element.
    aot_index: HashMap<String, usize>,
}

impl Parser {
    fn new(t: Vec<TokenWithPos>) -> Self {
        Parser { tokens: t, pos: 0, meta: HashMap::new(), aot_index: HashMap::new() }
    }
    fn cur(&self) -> &Token { &self.tokens[self.pos].token }
    fn curpos(&self) -> (usize,usize) { (self.tokens[self.pos].line, self.tokens[self.pos].col) }
    fn cur_start(&self) -> usize { self.tokens[self.pos].start }
    fn prev_end(&self) -> usize { if self.pos == 0 { 0 } else { self.tokens[self.pos-1].end } }
    fn adv(&mut self) -> Token { let t=self.tokens[self.pos].token.clone(); if self.pos<self.tokens.len()-1 {self.pos+=1;} t }
    fn skipws(&mut self) { while self.pos<self.tokens.len(){match self.cur(){Token::Whitespace|Token::Comment(_)=>{self.adv();}_=>{break;}}} }
    fn skipnl(&mut self) { while self.pos<self.tokens.len(){match self.cur(){Token::Whitespace|Token::Comment(_)|Token::Newline=>{self.adv();}_=>{break;}}} }

    fn dup(&self, key: &str) -> ParseError {
        let (line, col) = self.curpos();
        ParseError::DuplicateKey { line, col, key: key.to_string() }
    }

    fn parse_document(&mut self) -> Result<Value, ParseError> {
        let mut root = BTreeMap::new();
        // Canonical prefix and key path of the table currently being filled.
        let mut cur_canon = String::new();
        let mut cur_path: Vec<String> = Vec::new();
        self.skipnl();
        while !matches!(self.cur(), Token::Eof) {
            self.skipnl();
            if matches!(self.cur(), Token::Eof) { break; }
            match self.cur() {
                Token::LeftBracket => {
                    let (path, is_arr) = self.parse_table_header()?;
                    cur_canon = if is_arr {
                        self.resolve_aot_header(&mut root, &path)?
                    } else {
                        self.resolve_header(&mut root, &path)?
                    };
                    cur_path = path;
                }
                _ => {
                    let (kp, val) = self.parse_kv(&cur_canon)?;
                    let tgt = navigate(&mut root, &cur_path)?;
                    self.insert_kv(tgt, &cur_canon, &kp, val, false)?;
                    // A key/value pair owns the rest of its line.
                    self.skipws();
                    match self.cur() {
                        Token::Newline | Token::Eof | Token::Comment(_) => {}
                        _ => return Err(self.err_expected("newline after key/value pair")),
                    }
                }
            }
            self.skipnl();
        }
        Ok(Value::Table(root))
    }

    // ----- table headers -------------------------------------------------

    /// Walk `path`, registering intermediate tables, and return the canonical
    /// prefix for the table body that follows.
    fn resolve_header(&mut self, root: &mut BTreeMap<String, Value>, path: &[String]) -> Result<String, ParseError> {
        let mut canon = String::new();
        for (i, seg) in path.iter().enumerate() {
            let last = i == path.len() - 1;
            canon.push(SEP);
            canon.push_str(seg);
            match self.meta.get(&canon).copied() {
                Some(Kind::Aot) => {
                    let idx = *self.aot_index.get(&canon).unwrap_or(&0);
                    if last {
                        // `[a]` cannot reopen an array of tables.
                        return Err(self.dup(seg));
                    }
                    canon.push(IDX);
                    canon.push_str(&idx.to_string());
                }
                Some(Kind::Header) if last => return Err(self.dup(seg)),
                Some(Kind::Header) | Some(Kind::Implicit) => {
                    if last { self.meta.insert(canon.clone(), Kind::Header); }
                }
                // A table created by a dotted key may gain deeper sub-tables
                // (`[fruit.apple.texture]`) but may not be redefined by its
                // own header (`[fruit.apple]`).
                Some(Kind::Dotted) if !last => {}
                Some(_) => return Err(self.dup(seg)),
                None => {
                    self.meta.insert(canon.clone(), if last { Kind::Header } else { Kind::Implicit });
                }
            }
        }
        let _ = navigate(root, path)?;
        Ok(canon)
    }

    /// Same, for `[[array]]`: the final segment appends a fresh element.
    fn resolve_aot_header(&mut self, root: &mut BTreeMap<String, Value>, path: &[String]) -> Result<String, ParseError> {
        let mut canon = String::new();
        for (i, seg) in path.iter().enumerate() {
            let last = i == path.len() - 1;
            canon.push(SEP);
            canon.push_str(seg);
            if !last {
                match self.meta.get(&canon).copied() {
                    Some(Kind::Aot) => {
                        let idx = *self.aot_index.get(&canon).unwrap_or(&0);
                        canon.push(IDX);
                        canon.push_str(&idx.to_string());
                    }
                    Some(Kind::Header) | Some(Kind::Implicit) | Some(Kind::Dotted) => {}
                    Some(_) => return Err(self.dup(seg)),
                    None => { self.meta.insert(canon.clone(), Kind::Implicit); }
                }
                continue;
            }
            match self.meta.get(&canon).copied() {
                Some(Kind::Aot) => {
                    let idx = self.aot_index.entry(canon.clone()).or_insert(0);
                    *idx += 1;
                    let idx = *idx;
                    canon.push(IDX);
                    canon.push_str(&idx.to_string());
                }
                Some(_) => return Err(self.dup(seg)),
                None => {
                    self.meta.insert(canon.clone(), Kind::Aot);
                    self.aot_index.insert(canon.clone(), 0);
                    canon.push(IDX);
                    canon.push('0');
                }
            }
        }

        // Append the element to the value tree.
        let parent = navigate(root, &path[..path.len()-1])?;
        let key = &path[path.len()-1];
        let entry = parent.entry(key.clone()).or_insert_with(|| Value::Array(Vec::new()));
        match entry {
            Value::Array(a) => a.push(Value::Table(BTreeMap::new())),
            _ => return Err(self.dup(key)),
        }
        Ok(canon)
    }

    fn parse_table_header(&mut self) -> Result<(Vec<String>, bool), ParseError> {
        let open_end = self.tokens[self.pos].end;
        self.adv(); // [
        // `[[` only means "array of tables" when the brackets are adjacent;
        // `[ [table]]` is a malformed header, not an AOT.
        let is_arr = matches!(self.cur(), Token::LeftBracket) && self.cur_start() == open_end;
        if is_arr { self.adv(); }
        let mut path = Vec::new();
        self.skipws();
        loop {
            path.push(self.parse_key()?);
            self.skipws();
            if matches!(self.cur(), Token::Dot) { self.adv(); self.skipws(); } else { break; }
        }
        self.skipws();
        if is_arr {
            if !matches!(self.cur(), Token::RightBracket) { return Err(self.err_expected("]")); }
            let first_end = self.tokens[self.pos].end;
            self.adv();
            if !matches!(self.cur(), Token::RightBracket) || self.cur_start() != first_end {
                return Err(self.err_expected("]] with no space between brackets"));
            }
            self.adv();
        } else {
            if !matches!(self.cur(), Token::RightBracket) { return Err(self.err_expected("]")); }
            self.adv();
        }
        let _ = self.prev_end();
        self.skipws();
        match self.cur() {
            Token::Newline | Token::Eof | Token::Comment(_) => {}
            _ => { return Err(self.err_expected("newline")); }
        }
        Ok((path, is_arr))
    }

    // ----- keys and values -----------------------------------------------

    fn parse_key(&mut self) -> Result<String, ParseError> {
        let (l, c) = self.curpos();
        match self.adv() {
            Token::BareKey(s) => Ok(s),
            Token::String(s) => Ok(s),
            Token::MultilineString(_) => Err(ParseError::InvalidKey {
                line: l, col: c,
                message: "multi-line strings are not valid keys",
                got: "\"\"\"".to_string(),
            }),
            other => Err(ParseError::UnexpectedToken{line:l,col:c,expected:"key",got:format!("{:?}",other)}),
        }
    }

    fn parse_kv(&mut self, base: &str) -> Result<(Vec<String>, Value), ParseError> {
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
        let canon = canon_join(base, &kp);
        let v = self.parse_value(&canon)?;
        Ok((kp, v))
    }

    /// `base` is the canonical path this value will be stored at — inline
    /// tables need it so their own members can be registered.
    fn parse_value(&mut self, base: &str) -> Result<Value, ParseError> {
        let (l, c) = self.curpos();
        match self.adv() {
            Token::String(s) | Token::MultilineString(s) => Ok(Value::String(s)),
            Token::Integer(n) => Ok(Value::Integer(n)),
            Token::Float(f, orig) => Ok(Value::Float(f, orig)),
            Token::Boolean(b) => Ok(Value::Boolean(b)),
            Token::Datetime(s) => Ok(Value::Datetime(parse_datetime(&s, l, c)?)),
            Token::BareKey(s) => Err(ParseError::InvalidValue {
                line: l, col: c,
                message: format!("invalid value: {}", s),
            }),
            Token::LeftBracket => self.parse_array(base),
            Token::LeftBrace => self.parse_inline_table(base),
            other => Err(ParseError::UnexpectedToken{line:l,col:c,expected:"value",got:format!("{:?}",other)}),
        }
    }

    fn parse_array(&mut self, base: &str) -> Result<Value, ParseError> {
        let mut arr = Vec::new();
        self.skipnl();
        if matches!(self.cur(), Token::RightBracket) { self.adv(); return Ok(Value::Array(arr)); }
        loop {
            self.skipnl();
            if matches!(self.cur(), Token::RightBracket) { self.adv(); return Ok(Value::Array(arr)); }
            // Each element gets its own canonical slot so that two inline
            // tables in one array can't be mistaken for each other.
            let elem_base = format!("{}{}#{}", base, IDX, arr.len());
            let v = self.parse_value(&elem_base)?;
            arr.push(v);
            self.skipnl();
            match self.cur() {
                Token::Comma => { self.adv(); self.skipnl(); }
                Token::RightBracket => { self.adv(); return Ok(Value::Array(arr)); }
                _ => return Err(self.err_expected(", or ]")),
            }
        }
    }

    fn parse_inline_table(&mut self, base: &str) -> Result<Value, ParseError> {
        let mut tbl = BTreeMap::new();
        // Newlines and a trailing comma are permitted as of TOML 1.1.
        self.skipnl();
        if matches!(self.cur(), Token::RightBrace) { self.adv(); return Ok(Value::Table(tbl)); }
        loop {
            self.skipnl();
            let mut kp = Vec::new();
            loop {
                kp.push(self.parse_key()?);
                self.skipws();
                if matches!(self.cur(), Token::Dot) { self.adv(); self.skipws(); } else { break; }
            }
            if !matches!(self.cur(), Token::Equals) { return Err(self.err_expected("=")); }
            self.adv(); self.skipws();
            let canon = canon_join(base, &kp);
            let v = self.parse_value(&canon)?;
            self.insert_kv(&mut tbl, base, &kp, v, true)?;
            self.skipnl();
            match self.cur() {
                Token::Comma => {
                    self.adv();
                    self.skipnl();
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

    // ----- insertion -----------------------------------------------------

    /// Insert `val` at `path` inside `tbl`, registering every path it creates.
    fn insert_kv(
        &mut self,
        tbl: &mut BTreeMap<String, Value>,
        base: &str,
        path: &[String],
        val: Value,
        inline: bool,
    ) -> Result<(), ParseError> {
        let super_kind = if inline { Kind::InlineDotted } else { Kind::Dotted };
        let mut canon = base.to_string();
        for seg in &path[..path.len()-1] {
            canon.push(SEP);
            canon.push_str(seg);
            match self.meta.get(&canon).copied() {
                None => { self.meta.insert(canon.clone(), super_kind); }
                Some(k) if k == super_kind => {}
                // Anything else here is an attempt to extend a table that is
                // already closed: a value, an inline table, an AOT, or a
                // table defined by its own `[header]`.
                Some(_) => return Err(self.dup(seg)),
            }
        }
        let leaf = &path[path.len()-1];
        canon.push(SEP);
        canon.push_str(leaf);
        if self.meta.contains_key(&canon) {
            return Err(self.dup(leaf));
        }
        self.meta.insert(canon, if matches!(val, Value::Table(_)) { Kind::Inline } else { Kind::Value });

        let target = navigate_creating(tbl, &path[..path.len()-1])?;
        target.insert(leaf.clone(), val);
        Ok(())
    }

    fn err_expected(&self, exp: &'static str) -> ParseError {
        let (l, c) = self.curpos();
        ParseError::ExpectedToken{line:l,col:c,expected:exp,got:format!("{:?}", self.cur())}
    }
}

fn canon_join(base: &str, path: &[String]) -> String {
    let mut s = base.to_string();
    for seg in path {
        s.push(SEP);
        s.push_str(seg);
    }
    s
}

fn internal(what: &str) -> ParseError {
    ParseError::InvalidValue { line: 0, col: 0, message: what.to_string() }
}

/// Walk an already-validated path, following arrays of tables to their last
/// element. Only reached after `meta` has approved the path.
fn navigate<'a>(root: &'a mut BTreeMap<String, Value>, path: &[String]) -> Result<&'a mut BTreeMap<String, Value>, ParseError> {
    let mut cur = root;
    for key in path {
        let entry = cur.entry(key.clone()).or_insert_with(|| Value::Table(BTreeMap::new()));
        cur = match entry {
            Value::Table(t) => t,
            Value::Array(a) => match a.last_mut() {
                Some(Value::Table(t)) => t,
                _ => return Err(internal("cannot descend into a non-table array")),
            },
            _ => return Err(internal("cannot descend into a non-table value")),
        };
    }
    Ok(cur)
}

/// Same, but for dotted-key super-tables, which are always plain tables.
fn navigate_creating<'a>(tbl: &'a mut BTreeMap<String, Value>, path: &[String]) -> Result<&'a mut BTreeMap<String, Value>, ParseError> {
    let mut cur = tbl;
    for key in path {
        let entry = cur.entry(key.clone()).or_insert_with(|| Value::Table(BTreeMap::new()));
        cur = match entry {
            Value::Table(t) => t,
            _ => return Err(internal("cannot descend into a non-table value")),
        };
    }
    Ok(cur)
}
