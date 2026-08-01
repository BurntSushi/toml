//! Lexer — ported from lex.go (1248 LOC)
//! Tokenizes TOML input into a stream of tokens.
//! The lexer is context-aware: after = or , or [ it produces value tokens
//! that include dots; in key position it produces bare key tokens.

use crate::error::ParseError;

#[derive(Debug, Clone, PartialEq)]
pub enum Token {
    Eof, Newline, Whitespace,
    LeftBracket, RightBracket, LeftBrace, RightBrace,
    Comma, Dot, Equals,
    String(String),
    BareKey(String),
    Integer(i64),
    Float(f64),
    Boolean(bool),
    Datetime(String),
    Comment(String),
}

#[derive(Debug, Clone, PartialEq)]
pub struct TokenWithPos {
    pub token: Token,
    pub line: usize,
    pub col: usize,
    pub start: usize,
    pub end: usize,
}

/// Lex a TOML string into a vector of tokens with positions.
pub fn lex(input: &str) -> Result<Vec<TokenWithPos>, ParseError> {
    let input = input.strip_prefix('\u{FEFF}').unwrap_or(input);
    let mut tokens = Vec::new();
    let chars: Vec<char> = input.chars().collect();
    let mut pos = 0;
    let mut line = 1;
    let mut col = 1;

    while pos < chars.len() {
        let c = chars[pos];

        match c {
            ' ' | '\t' => { while pos < chars.len() && (chars[pos]==' '||chars[pos]=='\t') { pos+=1; col+=1; } }
            '\n' => { tokens.push(TokenWithPos{token:Token::Newline,line,col,start:pos,end:pos+1}); pos+=1; line+=1; col=1; }
            '\r' => { pos+=1; if pos<chars.len()&&chars[pos]=='\n'{pos+=1;} tokens.push(TokenWithPos{token:Token::Newline,line,col,start:pos-2,end:pos}); line+=1; col=1; }
            '#' => {
                let s=pos; pos+=1; let mut c2=String::new();
                while pos<chars.len()&&chars[pos]!='\n'&&chars[pos]!='\r' { c2.push(chars[pos]); pos+=1; col+=1; }
                tokens.push(TokenWithPos{token:Token::Comment(c2.trim().to_string()),line,col,start:s,end:pos});
            }
            '[' => { tokens.push(TokenWithPos{token:Token::LeftBracket,line,col,start:pos,end:pos+1}); pos+=1; col+=1; }
            ']' => { tokens.push(TokenWithPos{token:Token::RightBracket,line,col,start:pos,end:pos+1}); pos+=1; col+=1; }
            '{' => { tokens.push(TokenWithPos{token:Token::LeftBrace,line,col,start:pos,end:pos+1}); pos+=1; col+=1; }
            '}' => { tokens.push(TokenWithPos{token:Token::RightBrace,line,col,start:pos,end:pos+1}); pos+=1; col+=1; }
            ',' => { tokens.push(TokenWithPos{token:Token::Comma,line,col,start:pos,end:pos+1}); pos+=1; col+=1; }
            '=' => { tokens.push(TokenWithPos{token:Token::Equals,line,col,start:pos,end:pos+1}); pos+=1; col+=1; }
            '"' => { let(t,np,nl,nc)=lex_string(&chars,pos,line,col)?; tokens.push(TokenWithPos{token:t,line,col,start:pos,end:np}); line+=nl; col=nc; pos=np; }
            '\'' => { let(t,np,nl,nc)=lex_literal(&chars,pos,line,col)?; tokens.push(TokenWithPos{token:t,line,col,start:pos,end:np}); line+=nl; col=nc; pos=np; }
            '.' => { tokens.push(TokenWithPos{token:Token::Dot,line,col,start:pos,end:pos+1}); pos+=1; col+=1; }
            // Control characters are invalid in TOML
            c if (c as u32) < 0x20 && c != '\t' => {
                return Err(ParseError::UnexpectedChar { line, col, char: c });
            }
            c if (c as u32) == 0x7F => {
                return Err(ParseError::UnexpectedChar { line, col, char: c });
            }
            _ => {
                // Context-aware lexing: if the previous significant token was '=' or ',' or '[' or '{',
                // we're in value position — include dots in the token.
                let in_value_position = is_value_position(&tokens);

                let (tok, new_pos) = if in_value_position {
                    lex_value_token(&chars, pos, line, col)?
                } else {
                    lex_key_token(&chars, pos, line, col)?
                };
                let len = new_pos - pos;
                tokens.push(TokenWithPos{token:tok,line,col,start:pos,end:new_pos});
                pos = new_pos;
                col += len;
            }
        }
    }
    tokens.push(TokenWithPos{token:Token::Eof,line,col,start:pos,end:pos});
    Ok(tokens)
}

/// Check if the previous significant token indicates we're in value position.
/// After '=' or ',' (outside braces) or '[' we expect a value.
/// After '{' or ',' inside braces, we expect a KEY.
fn is_value_position(tokens: &[TokenWithPos]) -> bool {
    let mut brace_depth = 0i32;
    let mut bracket_depth = 0i32;
    let mut last_significant = &Token::Eof as &Token;
    for t in tokens.iter() {
        match &t.token {
            Token::Whitespace | Token::Comment(_) | Token::Newline => continue,
            Token::LeftBrace => {
                brace_depth += 1;
                last_significant = &Token::RightBrace;
            }
            Token::RightBrace => { brace_depth -= 1; last_significant = &t.token; }
            Token::LeftBracket => {
                bracket_depth += 1;
                let is_array = matches!(last_significant, Token::Equals | Token::Comma | Token::LeftBracket);
                if is_array {
                    last_significant = &t.token;
                } else {
                    last_significant = &Token::RightBrace;
                }
            }
            Token::RightBracket => { bracket_depth -= 1; last_significant = &t.token; }
            Token::Equals => { last_significant = &t.token; }
            Token::Comma => {
                // If inside brackets (array), comma is always value separator
                if bracket_depth > 0 {
                    last_significant = &t.token;
                } else if brace_depth > 0 {
                    last_significant = &Token::RightBrace;
                } else {
                    last_significant = &t.token;
                }
            }
            other => { last_significant = other; }
        }
    }
    matches!(last_significant, Token::Equals | Token::Comma | Token::LeftBracket)
}

/// Lex a key token — breaks on dots (they're separate tokens for dotted keys)
fn lex_key_token(chars: &[char], start: usize, line: usize, col: usize) -> Result<(Token, usize), ParseError> {
    let mut pos = start;
    let mut buf = String::new();
    while pos < chars.len() {
        let c = chars[pos];
        if c == ' ' || c == '\t' || c == '\n' || c == '\r' || c == ',' || c == ']'
            || c == '}' || c == '#' || c == '=' || c == '.'
        {
            break;
        }
        buf.push(c);
        pos += 1;
    }
    if buf.is_empty() {
        return Err(ParseError::UnexpectedChar { line, col, char: chars[start] });
    }
    // Keys are always BareKey (even if they look like dates or numbers)
    Ok((Token::BareKey(buf), pos))
}

/// Lex a value token — includes dots and exponents for floats/datetimes.
/// Also handles space-separated datetimes (1987-07-05 17:45:00Z).
fn lex_value_token(chars: &[char], start: usize, line: usize, col: usize) -> Result<(Token, usize), ParseError> {
    let mut pos = start;
    let mut buf = String::new();
    while pos < chars.len() {
        let c = chars[pos];
        if c == ' ' || c == '\t' || c == '\n' || c == '\r' || c == ',' || c == ']' || c == '}' || c == '#' {
            break;
        }
        buf.push(c);
        pos += 1;
    }
    if buf.is_empty() {
        return Err(ParseError::UnexpectedChar { line, col, char: chars[start] });
    }

    // Check for space-separated datetime: "1987-07-05 17:45:00Z"
    // If the current token looks like a date (YYYY-MM-DD) and is followed by
    // a space and then a time (HH:MM:SS...), consume both as one token.
    if looks_like_dt(&buf) && !buf.contains('T') && !buf.contains(':') {
        // Skip whitespace
        let mut peek = pos;
        while peek < chars.len() && (chars[peek] == ' ' || chars[peek] == '\t') {
            peek += 1;
        }
        // Check if the next part looks like a time
        if peek < chars.len() && peek != pos {
            let mut time_buf = String::new();
            let mut peek2 = peek;
            while peek2 < chars.len() {
                let c = chars[peek2];
                if c == ' ' || c == '\t' || c == '\n' || c == '\r' || c == ',' || c == ']' || c == '}' || c == '#' {
                    break;
                }
                time_buf.push(c);
                peek2 += 1;
            }
            if time_buf.len() >= 5
                && time_buf.chars().nth(0).map_or(false, |c| c.is_ascii_digit())
                && time_buf.chars().nth(1).map_or(false, |c| c.is_ascii_digit())
                && time_buf.chars().nth(2) == Some(':')
            {
                // It's a space-separated datetime! Consume both parts.
                buf.push(' ');
                buf.push_str(&time_buf);
                pos = peek2;
            }
        }
    }

    classify_value(&buf, pos)
}

fn classify_value(buf: &str, pos: usize) -> Result<(Token, usize), ParseError> {
    if buf == "true" { return Ok((Token::Boolean(true), pos)); }
    if buf == "false" { return Ok((Token::Boolean(false), pos)); }
    if buf == "inf" || buf == "+inf" { return Ok((Token::Float(f64::INFINITY), pos)); }
    if buf == "-inf" { return Ok((Token::Float(f64::NEG_INFINITY), pos)); }
    if buf == "nan" || buf == "+nan" || buf == "-nan" { return Ok((Token::Float(f64::NAN), pos)); }
    if let Ok(n) = parse_int(buf) { return Ok((Token::Integer(n), pos)); }
    // Try parsing as float — strip underscores first
    let cleaned: String = buf.chars().filter(|c| *c != '_').collect();
    if let Ok(f) = cleaned.parse::<f64>() { return Ok((Token::Float(f), pos)); }
    if looks_like_dt(buf) { return Ok((Token::Datetime(buf.to_string()), pos)); }
    // Fallback: treat as bare key (shouldn't happen for valid TOML)
    Ok((Token::BareKey(buf.to_string()), pos))
}

fn parse_int(s: &str) -> Result<i64, ()> {
    let c: String = s.chars().filter(|c| *c != '_').collect();
    if c.starts_with("0x") { i64::from_str_radix(&c[2..], 16).map_err(|_| ()) }
    else if c.starts_with("0o") { i64::from_str_radix(&c[2..], 8).map_err(|_| ()) }
    else if c.starts_with("0b") { i64::from_str_radix(&c[2..], 2).map_err(|_| ()) }
    else { c.parse::<i64>().map_err(|_| ()) }
}

fn looks_like_dt(s: &str) -> bool {
    let ch: Vec<char> = s.chars().collect();
    if ch.len() >= 8 && ch[0].is_ascii_digit() && ch[1].is_ascii_digit()
        && ch[2].is_ascii_digit() && ch[3].is_ascii_digit() && ch[4] == '-'
    { return true; }
    if ch.len() >= 5 && ch[0].is_ascii_digit() && ch[1].is_ascii_digit() && ch[2] == ':'
    { return true; }
    false
}

fn is_triple(chars:&[char],pos:usize,q:char)->bool{ pos+2<chars.len()&&chars[pos]==q&&chars[pos+1]==q&&chars[pos+2]==q }

fn lex_string(chars:&[char],start:usize,line:usize,col:usize)->Result<(Token,usize,usize,usize),ParseError>{
    if is_triple(chars,start,'"'){return lex_multi(chars,start,line,col,'"',true);}
    let mut pos=start+1; let mut r=String::new();
    while pos<chars.len() {
        let c=chars[pos];
        if c=='"'{return Ok((Token::String(r),pos+1,0,col+(pos-start)+1));}
        if c=='\n'{return Err(ParseError::UnterminatedString{line,col});}
        if c=='\\'{
            pos+=1; if pos>=chars.len(){return Err(ParseError::UnexpectedEof{line,col});}
            match chars[pos]{
                'n'=>r.push('\n'),'t'=>r.push('\t'),'r'=>r.push('\r'),
                '"'=>r.push('"'),'\\'=>r.push('\\'),'b'=>r.push('\u{0008}'),'f'=>r.push('\u{000C}'),
                'u'=>{if pos+4>=chars.len(){return Err(ParseError::UnexpectedEof{line,col});}let h:String=chars[pos+1..pos+5].iter().collect();let c=u32::from_str_radix(&h,16).map_err(|_|ParseError::InvalidEscape{line,col})?;if let Some(ch)=char::from_u32(c){r.push(ch);}pos+=4;}
                'U'=>{if pos+8>=chars.len(){return Err(ParseError::UnexpectedEof{line,col});}let h:String=chars[pos+1..pos+9].iter().collect();let c=u32::from_str_radix(&h,16).map_err(|_|ParseError::InvalidEscape{line,col})?;if let Some(ch)=char::from_u32(c){r.push(ch);}pos+=8;}
                _=>return Err(ParseError::InvalidEscape{line,col}),
            }
            pos+=1;
        } else {
            if c!='\t'&&(c as u32)<0x20{return Err(ParseError::UnexpectedChar{line,col,char:c});}
            r.push(c); pos+=1;
        }
    }
    Err(ParseError::UnterminatedString{line,col})
}

fn lex_literal(chars:&[char],start:usize,line:usize,col:usize)->Result<(Token,usize,usize,usize),ParseError>{
    if is_triple(chars,start,'\''){return lex_multi(chars,start,line,col,'\'',false);}
    let mut pos=start+1; let mut r=String::new();
    while pos<chars.len() {
        let c=chars[pos];
        if c=='\''{return Ok((Token::String(r),pos+1,0,col+(pos-start)+1));}
        if c=='\n'{return Err(ParseError::UnterminatedString{line,col});}
        if c!='\t'&&(c as u32)<0x20{return Err(ParseError::UnexpectedChar{line,col,char:c});}
        r.push(c); pos+=1;
    }
    Err(ParseError::UnterminatedString{line,col})
}

fn lex_multi(chars:&[char],start:usize,line:usize,col:usize,quote:char,esc:bool)->Result<(Token,usize,usize,usize),ParseError>{
    let mut pos=start+3; let mut r=String::new(); let mut nl=0;
    if pos<chars.len()&&chars[pos]=='\r'{pos+=1; if pos<chars.len()&&chars[pos]=='\n'{pos+=1;} nl+=1;}
    else if pos<chars.len()&&chars[pos]=='\n'{pos+=1; nl+=1;}
    while pos<chars.len() {
        if is_triple(chars,pos,quote){let nc=if nl>0{col+3}else{col+(pos-start)+3};return Ok((Token::String(r),pos+3,nl,nc));}
        let c=chars[pos];
        if esc&&c=='\\'{
            if pos+1<chars.len()&&(chars[pos+1]=='\n'||chars[pos+1]=='\r'){pos+=1; while pos<chars.len()&&(chars[pos]==' '||chars[pos]=='\t'||chars[pos]=='\n'||chars[pos]=='\r'){if chars[pos]=='\n'{nl+=1;}pos+=1;} continue;}
            pos+=1; if pos>=chars.len(){return Err(ParseError::UnexpectedEof{line,col});}
            match chars[pos]{
                'n'=>r.push('\n'),'t'=>r.push('\t'),'r'=>r.push('\r'),'"'=>r.push('"'),'\''=>r.push('\''),'\\'=>r.push('\\'),'b'=>r.push('\u{0008}'),'f'=>r.push('\u{000C}'),
                'u'=>{if pos+4>=chars.len(){return Err(ParseError::UnexpectedEof{line,col});}let h:String=chars[pos+1..pos+5].iter().collect();let cd=u32::from_str_radix(&h,16).map_err(|_|ParseError::InvalidEscape{line,col})?;if let Some(ch)=char::from_u32(cd){r.push(ch);}pos+=4;}
                'U'=>{if pos+8>=chars.len(){return Err(ParseError::UnexpectedEof{line,col});}let h:String=chars[pos+1..pos+9].iter().collect();let cd=u32::from_str_radix(&h,16).map_err(|_|ParseError::InvalidEscape{line,col})?;if let Some(ch)=char::from_u32(cd){r.push(ch);}pos+=8;}
                _=>return Err(ParseError::InvalidEscape{line,col}),
            }
            pos+=1;
        } else {
            if c=='\n'{nl+=1;}
            r.push(c); pos+=1;
        }
    }
    Err(ParseError::UnterminatedString{line,col})
}