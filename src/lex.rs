//! Lexer — ported from lex.go (1248 LOC)
//! Tokenizes TOML input into a stream of tokens.
//! The lexer is context-aware: after = or , or [ it produces value tokens
//! that include dots; in key position it produces bare key tokens.

use crate::error::ParseError;
use crate::number::{parse_number, Number};

#[derive(Debug, Clone, PartialEq)]
pub enum Token {
    Eof, Newline, Whitespace,
    LeftBracket, RightBracket, LeftBrace, RightBrace,
    Comma, Dot, Equals,
    String(String),
    /// A `"""…"""` or `'''…'''` string. Tracked separately from `String`
    /// because multi-line strings are values only — never keys.
    MultilineString(String),
    BareKey(String),
    Integer(i64),
    Float(f64, String),
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

/// Control characters are banned everywhere except as an explicit tab or
/// newline: U+0000–U+0008, U+000A–U+001F (context permitting), and U+007F.
fn is_banned_control(c: char) -> bool {
    let n = c as u32;
    (n < 0x20 && c != '\t' && c != '\n' && c != '\r') || n == 0x7F
}

/// Bare keys are ASCII letters, digits, underscore, and dash — nothing else.
fn is_bare_key_char(c: char) -> bool {
    c.is_ascii_alphanumeric() || c == '_' || c == '-'
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
            '\r' => {
                // A carriage return is only legal as part of a CRLF pair.
                if pos + 1 >= chars.len() || chars[pos+1] != '\n' {
                    return Err(ParseError::UnexpectedChar { line, col, char: '\r' });
                }
                tokens.push(TokenWithPos{token:Token::Newline,line,col,start:pos,end:pos+2});
                pos += 2; line += 1; col = 1;
            }
            '#' => {
                let s=pos; pos+=1; col+=1; let mut c2=String::new();
                while pos<chars.len()&&chars[pos]!='\n'&&chars[pos]!='\r' {
                    if is_banned_control(chars[pos]) {
                        return Err(ParseError::UnexpectedChar { line, col, char: chars[pos] });
                    }
                    c2.push(chars[pos]); pos+=1; col+=1;
                }
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
            c if is_banned_control(c) => {
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

/// Lex a bare key — breaks on dots (they're separate tokens for dotted keys).
fn lex_key_token(chars: &[char], start: usize, line: usize, col: usize) -> Result<(Token, usize), ParseError> {
    let mut pos = start;
    let mut buf = String::new();
    while pos < chars.len() && is_bare_key_char(chars[pos]) {
        buf.push(chars[pos]);
        pos += 1;
    }
    if buf.is_empty() {
        return Err(ParseError::InvalidKey {
            line, col,
            message: "bare keys may only contain A-Z a-z 0-9 _ -",
            got: chars[start].to_string(),
        });
    }
    // A bare key ends at a delimiter; anything else here is a stray character
    // that would silently become part of the key (`bare!key`, `μ`, `\u00c0`).
    if pos < chars.len() && !is_key_delimiter(chars[pos]) {
        return Err(ParseError::InvalidKey {
            line, col,
            message: "bare keys may only contain A-Z a-z 0-9 _ -",
            got: chars[pos].to_string(),
        });
    }
    Ok((Token::BareKey(buf), pos))
}

fn is_key_delimiter(c: char) -> bool {
    matches!(c, ' ' | '\t' | '\n' | '\r' | ',' | ']' | '}' | '#' | '=' | '.')
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

    classify_value(&buf, pos, line, col)
}

fn classify_value(buf: &str, pos: usize, line: usize, col: usize) -> Result<(Token, usize), ParseError> {
    if buf == "true" { return Ok((Token::Boolean(true), pos)); }
    if buf == "false" { return Ok((Token::Boolean(false), pos)); }
    if buf == "inf" || buf == "+inf" { return Ok((Token::Float(f64::INFINITY, buf.to_string()), pos)); }
    if buf == "-inf" { return Ok((Token::Float(f64::NEG_INFINITY, buf.to_string()), pos)); }
    if buf == "nan" || buf == "+nan" { return Ok((Token::Float(f64::NAN, buf.to_string()), pos)); }
    if buf == "-nan" { return Ok((Token::Float(-f64::NAN, buf.to_string()), pos)); }

    // Datetimes are number-shaped too, so they get first refusal; the parser
    // validates the grammar once it knows this is a value and not a key.
    if looks_like_dt(buf) { return Ok((Token::Datetime(buf.to_string()), pos)); }

    if is_number_start(buf) {
        return match parse_number(buf, line, col)? {
            Number::Integer(n) => Ok((Token::Integer(n), pos)),
            Number::Float(f) => Ok((Token::Float(f, buf.to_string()), pos)),
        };
    }

    Ok((Token::BareKey(buf.to_string()), pos))
}

fn is_number_start(buf: &str) -> bool {
    let b = buf.as_bytes();
    if b.is_empty() { return false; }
    b[0].is_ascii_digit() || (b.len() > 1 && (b[0] == b'+' || b[0] == b'-') && b[1].is_ascii_digit())
}

/// Cheap shape test: a leading `YYYY-` or `HH:` means "treat as datetime and
/// let the grammar decide". Deliberately loose so that `1987-7-05` reaches the
/// datetime validator and gets a real error instead of falling through.
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

/// Decode `\uXXXX` / `\UXXXXXXXX`, rejecting surrogates and out-of-range
/// codepoints rather than silently dropping them.
fn unicode_escape(chars:&[char],pos:usize,width:usize,line:usize,col:usize)->Result<(char,usize),ParseError>{
    if pos+width>=chars.len(){return Err(ParseError::UnexpectedEof{line,col});}
    let h:String=chars[pos+1..pos+1+width].iter().collect();
    let cp=u32::from_str_radix(&h,16).map_err(|_|ParseError::InvalidEscape{line,col})?;
    let ch=char::from_u32(cp).ok_or(ParseError::InvalidEscape{line,col})?;
    Ok((ch,pos+width))
}

fn lex_string(chars:&[char],start:usize,line:usize,col:usize)->Result<(Token,usize,usize,usize),ParseError>{
    if is_triple(chars,start,'"'){return lex_multi(chars,start,line,col,'"',true);}
    let mut pos=start+1; let mut r=String::new();
    while pos<chars.len() {
        let c=chars[pos];
        if c=='"'{return Ok((Token::String(r),pos+1,0,col+(pos-start)+1));}
        if c=='\n'||c=='\r'{return Err(ParseError::UnterminatedString{line,col});}
        if c=='\\'{
            pos+=1; if pos>=chars.len(){return Err(ParseError::UnexpectedEof{line,col});}
            match chars[pos]{
                'n'=>r.push('\n'),'t'=>r.push('\t'),'r'=>r.push('\r'),
                '"'=>r.push('"'),'\\'=>r.push('\\'),'b'=>r.push('\u{0008}'),'f'=>r.push('\u{000C}'),
                'e'=>r.push('\u{001B}'),
                'x'=>{if pos+2>=chars.len(){return Err(ParseError::UnexpectedEof{line,col});}let h:String=chars[pos+1..pos+3].iter().collect();let c=u32::from_str_radix(&h,16).map_err(|_|ParseError::InvalidEscape{line,col})?;let ch=char::from_u32(c).ok_or(ParseError::InvalidEscape{line,col})?;r.push(ch);pos+=2;}
                'u'=>{let(ch,np)=unicode_escape(chars,pos,4,line,col)?;r.push(ch);pos=np;}
                'U'=>{let(ch,np)=unicode_escape(chars,pos,8,line,col)?;r.push(ch);pos=np;}
                _=>return Err(ParseError::InvalidEscape{line,col}),
            }
            pos+=1;
        } else {
            if is_banned_control(c){return Err(ParseError::UnexpectedChar{line,col,char:c});}
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
        if c=='\n'||c=='\r'{return Err(ParseError::UnterminatedString{line,col});}
        if is_banned_control(c){return Err(ParseError::UnexpectedChar{line,col,char:c});}
        r.push(c); pos+=1;
    }
    Err(ParseError::UnterminatedString{line,col})
}

fn lex_multi(chars:&[char],start:usize,line:usize,col:usize,quote:char,esc:bool)->Result<(Token,usize,usize,usize),ParseError>{
    let mut pos=start+3; let mut r=String::new(); let mut nl=0;
    // A newline immediately after the opening delimiter is trimmed.
    if pos<chars.len()&&chars[pos]=='\r'&&pos+1<chars.len()&&chars[pos+1]=='\n'{pos+=2; nl+=1;}
    else if pos<chars.len()&&chars[pos]=='\n'{pos+=1; nl+=1;}
    while pos<chars.len() {
        let c=chars[pos];

        if c==quote{
            // Up to two quotes may appear as content directly before the
            // closing delimiter; a run of six or more can't be parsed.
            let mut n=0; while pos+n<chars.len()&&chars[pos+n]==quote{n+=1;}
            if n>=3 {
                if n>5 {
                    return Err(ParseError::UnexpectedChar{line,col,char:quote});
                }
                for _ in 0..(n-3) { r.push(quote); }
                let nc=if nl>0{col+3}else{col+(pos-start)+n};
                return Ok((Token::MultilineString(r),pos+n,nl,nc));
            }
            for _ in 0..n { r.push(quote); }
            pos+=n;
            continue;
        }

        if esc&&c=='\\'{
            // Line continuation: a trailing `\` swallows the newline and all
            // leading whitespace on the next line. Whitespace between the `\`
            // and the newline is allowed; anything else is not.
            let mut peek=pos+1;
            while peek<chars.len()&&(chars[peek]==' '||chars[peek]=='\t'){peek+=1;}
            let at_newline = peek<chars.len()&&(chars[peek]=='\n'||chars[peek]=='\r');
            if at_newline {
                pos=peek;
                while pos<chars.len()&&(chars[pos]==' '||chars[pos]=='\t'||chars[pos]=='\n'||chars[pos]=='\r'){
                    if chars[pos]=='\n'{nl+=1;}
                    pos+=1;
                }
                continue;
            }
            if peek>pos+1 {
                // `\` followed by whitespace that does not end the line.
                return Err(ParseError::InvalidEscape{line,col});
            }

            pos+=1; if pos>=chars.len(){return Err(ParseError::UnexpectedEof{line,col});}
            match chars[pos]{
                'n'=>r.push('\n'),'t'=>r.push('\t'),'r'=>r.push('\r'),'"'=>r.push('"'),'\''=>r.push('\''),'\\'=>r.push('\\'),'b'=>r.push('\u{0008}'),'f'=>r.push('\u{000C}'),
                'e'=>r.push('\u{001B}'),
                'x'=>{if pos+2>=chars.len(){return Err(ParseError::UnexpectedEof{line,col});}let h:String=chars[pos+1..pos+3].iter().collect();let cd=u32::from_str_radix(&h,16).map_err(|_|ParseError::InvalidEscape{line,col})?;let ch=char::from_u32(cd).ok_or(ParseError::InvalidEscape{line,col})?;r.push(ch);pos+=2;}
                'u'=>{let(ch,np)=unicode_escape(chars,pos,4,line,col)?;r.push(ch);pos=np;}
                'U'=>{let(ch,np)=unicode_escape(chars,pos,8,line,col)?;r.push(ch);pos=np;}
                _=>return Err(ParseError::InvalidEscape{line,col}),
            }
            pos+=1;
            continue;
        }

        if c=='\r' {
            // Bare CR is illegal even inside a multi-line string.
            if pos+1>=chars.len()||chars[pos+1]!='\n'{
                return Err(ParseError::UnexpectedChar{line,col,char:'\r'});
            }
            r.push('\r'); r.push('\n'); pos+=2; nl+=1;
            continue;
        }
        if c=='\n'{nl+=1; r.push(c); pos+=1; continue;}
        if is_banned_control(c){return Err(ParseError::UnexpectedChar{line,col,char:c});}
        r.push(c); pos+=1;
    }
    Err(ParseError::UnterminatedString{line,col})
}
