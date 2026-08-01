//! Strict integer/float validation, following the TOML ABNF directly.
//!
//! The Go original leans on `strconv` after a permissive lexer pass, which lets
//! a handful of malformed literals (`1.e2`, `_1.2`, `0x_1`) slip through as
//! valid. Walking the grammar means the accept set is exactly the spec's.

use crate::error::ParseError;

pub enum Number {
    Integer(i64),
    Float(f64),
}

/// Validate and convert a numeric literal.
///
/// The caller has already established that `s` starts like a number
/// (sign or ASCII digit) and is not datetime-shaped.
pub fn parse_number(s: &str, line: usize, col: usize) -> Result<Number, ParseError> {
    let bad = |what: &'static str| ParseError::InvalidNumber {
        line,
        col,
        message: what,
        got: s.to_string(),
    };

    let b = s.as_bytes();
    let (sign_len, negative) = match b.first() {
        Some(b'+') => (1, false),
        Some(b'-') => (1, true),
        _ => (0, false),
    };
    let body = &b[sign_len..];
    if body.is_empty() {
        return Err(bad("no digits"));
    }

    // Radix-prefixed integers never carry a sign, a fraction, or an exponent.
    if body.len() >= 2 && body[0] == b'0' {
        let radix = match body[1] {
            b'x' => Some(16),
            b'o' => Some(8),
            b'b' => Some(2),
            _ => None,
        };
        if let Some(radix) = radix {
            if sign_len != 0 {
                return Err(bad("sign not allowed on prefixed integer"));
            }
            let digits = collect_underscored(&body[2..], |c| c.is_digit(radix))
                .ok_or_else(|| bad("malformed digits after radix prefix"))?;
            return i64::from_str_radix(&digits, radix)
                .map(Number::Integer)
                .map_err(|_| bad("integer out of range"));
        }
    }

    // Decimal integer part — shared by integers and floats.
    let int_len = underscored_len(body, |c| c.is_ascii_digit())
        .ok_or_else(|| bad("malformed integer part"))?;
    if int_len == 0 {
        return Err(bad("expected a digit"));
    }
    let int_part = &body[..int_len];
    // A leading zero is only legal when the integer part is exactly "0".
    if int_part.len() > 1 && int_part[0] == b'0' {
        return Err(bad("leading zeros are not allowed"));
    }

    let rest = &body[int_len..];
    if rest.is_empty() {
        let digits: String = strip_underscores(int_part);
        let signed = if negative {
            format!("-{}", digits)
        } else {
            digits
        };
        return signed
            .parse::<i64>()
            .map(Number::Integer)
            .map_err(|_| bad("integer out of range"));
    }

    // Anything past the integer part makes this a float: `frac [exp]` or `exp`.
    let mut idx = 0usize;
    let mut has_suffix = false;

    if rest[idx] == b'.' {
        idx += 1;
        // `zero-prefixable-int`: leading zeros fine here, but digits required.
        let n = underscored_len(&rest[idx..], |c| c.is_ascii_digit())
            .ok_or_else(|| bad("malformed fractional part"))?;
        if n == 0 {
            return Err(bad("expected a digit after the decimal point"));
        }
        idx += n;
        has_suffix = true;
    }

    if idx < rest.len() && (rest[idx] == b'e' || rest[idx] == b'E') {
        idx += 1;
        if idx < rest.len() && (rest[idx] == b'+' || rest[idx] == b'-') {
            idx += 1;
        }
        let n = underscored_len(&rest[idx..], |c| c.is_ascii_digit())
            .ok_or_else(|| bad("malformed exponent"))?;
        if n == 0 {
            return Err(bad("expected a digit in the exponent"));
        }
        idx += n;
        has_suffix = true;
    }

    if idx != rest.len() {
        return Err(bad("trailing characters after number"));
    }
    if !has_suffix {
        return Err(bad("expected a fraction or exponent"));
    }

    let cleaned: String = strip_underscores(body);
    let signed = if negative {
        format!("-{}", cleaned)
    } else {
        cleaned
    };
    let f: f64 = signed.parse().map_err(|_| bad("malformed float"))?;
    // A literal that overflows the f64 range is an error, not `inf` — only the
    // `inf` keyword may produce an infinity. (Underflow to zero is fine, and
    // matches the reference implementation.)
    if f.is_infinite() {
        return Err(bad("float out of range"));
    }
    Ok(Number::Float(f))
}

/// Length of a run of `pred` digits where every `_` sits between two digits.
/// Returns `None` if an underscore is misplaced.
fn underscored_len(b: &[u8], pred: impl Fn(char) -> bool) -> Option<usize> {
    let mut i = 0;
    let mut last_was_digit = false;
    while i < b.len() {
        let c = b[i] as char;
        if pred(c) {
            last_was_digit = true;
            i += 1;
        } else if c == '_' {
            // Must be preceded and followed by a digit.
            if !last_was_digit {
                return None;
            }
            if i + 1 >= b.len() || !pred(b[i + 1] as char) {
                return None;
            }
            last_was_digit = false;
            i += 1;
        } else {
            break;
        }
    }
    Some(i)
}

/// Like `underscored_len`, but requires the run to cover the whole slice.
fn collect_underscored(b: &[u8], pred: impl Fn(char) -> bool) -> Option<String> {
    let n = underscored_len(b, &pred)?;
    if n != b.len() || n == 0 {
        return None;
    }
    Some(strip_underscores(b))
}

fn strip_underscores(b: &[u8]) -> String {
    b.iter()
        .filter(|c| **c != b'_')
        .map(|c| *c as char)
        .collect()
}

/// Render a float the way Go's `strconv.FormatFloat(f, 'g', -1, 64)` does:
/// shortest round-tripping representation, switching to exponent form when the
/// decimal exponent falls outside [-4, 21).
pub fn format_float(f: f64) -> String {
    if f.is_nan() {
        return if f.is_sign_negative() { "-nan".into() } else { "nan".into() };
    }
    if f.is_infinite() {
        return if f > 0.0 { "inf".into() } else { "-inf".into() };
    }
    if f == 0.0 {
        return if f.is_sign_negative() { "-0".into() } else { "0".into() };
    }

    // Rust's `{:e}` already yields the shortest round-tripping mantissa.
    let sci = format!("{:e}", f);
    let (mantissa, exp) = sci.split_once('e').expect("{:e} always emits an exponent");
    let exp: i32 = exp.parse().expect("{:e} always emits a decimal exponent");

    if exp < -4 || exp >= 21 {
        let sign = if exp < 0 { '-' } else { '+' };
        return format!("{}e{}{:02}", mantissa, sign, exp.abs());
    }

    // Within Go's %f window: expand the mantissa by hand so we neither lose
    // precision nor pick up the trailing zeros that `{}` can introduce.
    let negative = mantissa.starts_with('-');
    let digits: String = mantissa.chars().filter(|c| c.is_ascii_digit()).collect();
    let mut out = String::new();
    if negative {
        out.push('-');
    }
    if exp >= 0 {
        let point = exp as usize + 1;
        if digits.len() <= point {
            out.push_str(&digits);
            out.push_str(&"0".repeat(point - digits.len()));
        } else {
            out.push_str(&digits[..point]);
            out.push('.');
            out.push_str(&digits[point..]);
        }
    } else {
        out.push_str("0.");
        out.push_str(&"0".repeat((-exp - 1) as usize));
        out.push_str(&digits);
    }
    out
}

#[cfg(test)]
mod tests {
    use super::*;

    fn int(s: &str) -> i64 {
        match parse_number(s, 1, 1).expect("should parse") {
            Number::Integer(n) => n,
            Number::Float(f) => panic!("{:?} parsed as float {}", s, f),
        }
    }
    fn float(s: &str) -> f64 {
        match parse_number(s, 1, 1).expect("should parse") {
            Number::Float(f) => f,
            Number::Integer(n) => panic!("{:?} parsed as integer {}", s, n),
        }
    }
    fn err(s: &str) {
        assert!(parse_number(s, 1, 1).is_err(), "{:?} should have been rejected", s);
    }

    #[test]
    fn parses_integers() {
        assert_eq!(int("0"), 0);
        assert_eq!(int("1_000"), 1000);
        assert_eq!(int("-17"), -17);
        assert_eq!(int("0xdead_beef"), 0xdeadbeef);
        assert_eq!(int("0o755"), 0o755);
        assert_eq!(int("0b1010"), 0b1010);
    }

    #[test]
    fn parses_floats() {
        assert_eq!(float("3.14"), 3.14);
        assert_eq!(float("3e1_4"), 3e14);
        assert_eq!(float("-0.01"), -0.01);
        assert_eq!(float("0.0"), 0.0);
    }

    #[test]
    fn rejects_malformed_numbers() {
        err("_1.2");     // leading underscore
        err("1.2_");     // trailing underscore
        err("1__2");     // doubled underscore
        err("01");       // leading zero
        err("1.");       // no digit after the point
        err("1.e2");     // fraction needs a digit
        err(".5");       // no integer part
        err("1e");       // exponent needs a digit
        err("1e_2");     // underscore beside the exponent
        err("0x_1");     // underscore after the radix prefix
        err("0x-1");     // sign on a prefixed integer
        err("1e999999");  // overflows to infinity
        err("99999999999999999999"); // overflows i64
    }

    #[test]
    fn formats_floats_like_go() {
        assert_eq!(format_float(1000.0), "1000");
        assert_eq!(format_float(3e14), "300000000000000");
        assert_eq!(format_float(5e22), "5e+22");
        assert_eq!(format_float(6.626e-34), "6.626e-34");
        assert_eq!(format_float(0.0001), "0.0001");
        assert_eq!(format_float(-0.0), "-0");
        assert_eq!(format_float(f64::INFINITY), "inf");
        assert_eq!(format_float(f64::NAN), "nan");
    }
}
