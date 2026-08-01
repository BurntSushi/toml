//! Lexer — ported from lex.go (1248 LOC)
//! Tokenizes TOML input into a stream of tokens.

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
            // Control characters (0x00-0x08, 0x0B, 0x0C, 0x0E-0x1F) are invalid in TOML
            c if (c as u32) < 0x20 && c != '\t' => {
                return Err(ParseError::UnexpectedChar { line, col, char: c });
            }
            c if (c as u32) == 0x7F => {
                return Err(ParseError::UnexpectedChar { line, col, char: c });
            }
            _ => {
                let (tok, new_pos) = lex_word(&chars, pos, line, col)?;
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

fn is_triple(chars:&[char],pos:usize,q:char)->bool{ pos+2<chars.len()&&chars[pos]==q&&chars[pos+1]==q&&chars[pos+2]==q }

fn lex_word(chars:&[char],start:usize,line:usize,col:usize)->Result<(Token,usize),ParseError>{
    let mut pos=start;
    let mut buf=String::new();
    while pos<chars.len() {
        let c=chars[pos];
        if c==' '||c=='\t'||c=='\n'||c=='\r'||c==','||c==']'||c=='}'||c=='#'||c=='='||c=='.' { break; }
        buf.push(c); pos+=1;
    }
    if buf.is_empty() { return Err(ParseError::UnexpectedChar{line,col,char:chars[start]}); }
    classify_word(&buf, pos)
}

/// Lex a word that may contain dots (for values like floats and datetimes)
fn lex_value_word(chars:&[char],start:usize,line:usize,col:usize)->Result<(Token,usize),ParseError>{
    let mut pos=start;
    let mut buf=String::new();
    while pos<chars.len() {
        let c=chars[pos];
        if c==' '||c=='\t'||c=='\n'||c=='\r'||c==','||c==']'||c=='}'||c=='#' { break; }
        buf.push(c); pos+=1;
    }
    if buf.is_empty() { return Err(ParseError::UnexpectedChar{line,col,char:chars[start]}); }
    classify_word(&buf, pos)
}

fn classify_word(buf:&str, pos:usize)->Result<(Token,usize),ParseError>{
    if buf=="true" { return Ok((Token::Boolean(true),pos)); }
    if buf=="false" { return Ok((Token::Boolean(false),pos)); }
    if let Ok(n)=parse_int(buf) { return Ok((Token::Integer(n),pos)); }
    if let Ok(f)=buf.parse::<f64>() { return Ok((Token::Float(f),pos)); }
    if buf=="inf"||buf=="+inf" { return Ok((Token::Float(f64::INFINITY),pos)); }
    if buf=="-inf" { return Ok((Token::Float(f64::NEG_INFINITY),pos)); }
    if buf=="nan"||buf=="+nan"||buf=="-nan" { return Ok((Token::Float(f64::NAN),pos)); }
    if looks_like_dt(buf) { return Ok((Token::Datetime(buf.to_string()),pos)); }
    // Bare key: letters, digits, hyphens, underscores, and non-ASCII
    Ok((Token::BareKey(buf.to_string()),pos))
}

fn parse_int(s:&str)->Result<i64,()> {
    let c:String=s.chars().filter(|c|*c!='_').collect();
    if c.starts_with("0x"){i64::from_str_radix(&c[2..],16).map_err(|_|())}
    else if c.starts_with("0o"){i64::from_str_radix(&c[2..],8).map_err(|_|())}
    else if c.starts_with("0b"){i64::from_str_radix(&c[2..],2).map_err(|_|())}
    else{c.parse::<i64>().map_err(|_|())}
}

fn looks_like_dt(s:&str)->bool{
    let ch:Vec<char>=s.chars().collect();
    if ch.len()>=8&&ch[0].is_ascii_digit()&&ch[1].is_ascii_digit()&&ch[2].is_ascii_digit()&&ch[3].is_ascii_digit()&&ch[4]=='-' {return true;}
    if ch.len()>=5&&ch[0].is_ascii_digit()&&ch[1].is_ascii_digit()&&ch[2]==':' {return true;}
    false
}

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