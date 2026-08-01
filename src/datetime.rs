//! Datetime parsing and validation.
//!
//! The Go original leans on `time.Parse` with a table of layout strings, which
//! silently accepts anything the layout happens to match. Here the RFC 3339 /
//! TOML ABNF grammar is walked explicitly, so every rejection has a reason and
//! the four TOML datetime kinds stay distinct in the type system.

use crate::error::ParseError;
use crate::{Date, Datetime, Time, TimeOffset};

/// Parse a TOML datetime literal, rejecting anything outside the ABNF grammar.
///
/// Accepts all four forms:
///   * offset date-time — `1979-05-27T07:32:00Z`, `1979-05-27 07:32:00-07:00`
///   * local date-time  — `1979-05-27T07:32:00`
///   * local date       — `1979-05-27`
///   * local time       — `07:32:00`
pub fn parse_datetime(s: &str, line: usize, col: usize) -> Result<Datetime, ParseError> {
    let b = s.as_bytes();
    let bad = |what: &'static str| ParseError::InvalidDatetime {
        line,
        col,
        message: what,
        got: s.to_string(),
    };

    // A leading `HH:` (and nothing date-shaped) means this is a local time.
    if b.len() >= 3 && b[2] == b':' {
        let (time, rest) = parse_time(b, line, col, s)?;
        if !rest.is_empty() {
            return Err(bad("trailing characters after local time"));
        }
        return Ok(Datetime::TimeOnly(time));
    }

    let date = parse_date(b, line, col, s)?;
    if b.len() == 10 {
        return Ok(Datetime::DateOnly(date));
    }

    // Date and time must be joined by `T`, `t`, or a single space.
    match b[10] {
        b'T' | b't' | b' ' => {}
        _ => return Err(bad("expected 'T' or space between date and time")),
    }

    let (time, rest) = parse_time(&b[11..], line, col, s)?;

    if rest.is_empty() {
        return Ok(Datetime::Local { date, time });
    }

    let offset = parse_offset(rest, line, col, s)?;
    Ok(Datetime::Offset { date, time, offset })
}

/// `YYYY-MM-DD`, with a real-calendar check on the day.
fn parse_date(b: &[u8], line: usize, col: usize, s: &str) -> Result<Date, ParseError> {
    let bad = |what: &'static str| ParseError::InvalidDatetime {
        line,
        col,
        message: what,
        got: s.to_string(),
    };
    if b.len() < 10 {
        return Err(bad("date too short, expected YYYY-MM-DD"));
    }
    if !(digits(&b[0..4]) && b[4] == b'-' && digits(&b[5..7]) && b[7] == b'-' && digits(&b[8..10])) {
        return Err(bad("malformed date, expected YYYY-MM-DD"));
    }

    let year = num(&b[0..4]) as i32;
    let month = num(&b[5..7]) as u8;
    let day = num(&b[8..10]) as u8;

    if month < 1 || month > 12 {
        return Err(bad("month out of range 01-12"));
    }
    if day < 1 || day > days_in_month(year, month) {
        return Err(bad("day out of range for month"));
    }
    Ok(Date { year, month, day })
}

/// `HH:MM:SS` with optional `.frac`. Returns the time and whatever follows it.
fn parse_time<'a>(
    b: &'a [u8],
    line: usize,
    col: usize,
    s: &str,
) -> Result<(Time, &'a [u8]), ParseError> {
    let bad = |what: &'static str| ParseError::InvalidDatetime {
        line,
        col,
        message: what,
        got: s.to_string(),
    };
    if b.len() < 5 {
        return Err(bad("time too short, expected HH:MM"));
    }
    if !(digits(&b[0..2]) && b[2] == b':' && digits(&b[3..5])) {
        return Err(bad("malformed time, expected HH:MM"));
    }

    let hour = num(&b[0..2]) as u8;
    let minute = num(&b[3..5]) as u8;

    // Seconds are optional as of TOML 1.1; when present they must be exactly
    // two digits, so `01:32:0` is still rejected.
    let mut consumed = 5;
    let mut second = 0u8;
    if b.len() > 5 && b[5] == b':' {
        if b.len() < 8 || !digits(&b[6..8]) {
            return Err(bad("malformed seconds, expected two digits"));
        }
        if b.len() > 8 && b[8].is_ascii_digit() {
            return Err(bad("malformed seconds, expected two digits"));
        }
        second = num(&b[6..8]) as u8;
        consumed = 8;
    }

    if hour > 23 {
        return Err(bad("hour out of range 00-23"));
    }
    if minute > 59 {
        return Err(bad("minute out of range 00-59"));
    }
    // 60 is allowed for leap seconds.
    if second > 60 {
        return Err(bad("second out of range 00-60"));
    }

    let mut rest = &b[consumed..];
    let mut nanosecond = 0u32;

    if !rest.is_empty() && rest[0] == b'.' {
        if consumed == 5 {
            return Err(bad("fractional seconds require a seconds field"));
        }
        let frac = &rest[1..];
        let n = frac.iter().take_while(|c| c.is_ascii_digit()).count();
        if n == 0 {
            return Err(bad("expected at least one digit after decimal point"));
        }
        // TOML allows arbitrary precision here; anything past nanoseconds is
        // truncated, matching Go's time.Time resolution.
        for (i, d) in frac[..n].iter().enumerate().take(9) {
            nanosecond += (d - b'0') as u32 * 10u32.pow(8 - i as u32);
        }
        rest = &frac[n..];
    }

    Ok((
        Time {
            hour,
            minute,
            second,
            nanosecond,
        },
        rest,
    ))
}

/// `Z`, `z`, or `±HH:MM`.
fn parse_offset(b: &[u8], line: usize, col: usize, s: &str) -> Result<TimeOffset, ParseError> {
    let bad = |what: &'static str| ParseError::InvalidDatetime {
        line,
        col,
        message: what,
        got: s.to_string(),
    };
    if b == b"Z" || b == b"z" {
        return Ok(TimeOffset::Z);
    }
    if b.len() != 6 || (b[0] != b'+' && b[0] != b'-') {
        return Err(bad("malformed offset, expected Z or +HH:MM"));
    }
    if !(digits(&b[1..3]) && b[3] == b':' && digits(&b[4..6])) {
        return Err(bad("malformed offset, expected Z or +HH:MM"));
    }

    let oh = num(&b[1..3]) as i16;
    let om = num(&b[4..6]) as i16;
    if oh > 23 {
        return Err(bad("offset hour out of range 00-23"));
    }
    if om > 59 {
        return Err(bad("offset minute out of range 00-59"));
    }

    let mins = oh * 60 + om;
    if mins == 0 {
        // Go normalises ±00:00 to Z, and so does the toml-test comparison.
        return Ok(TimeOffset::Z);
    }
    Ok(TimeOffset::Offset(if b[0] == b'-' { -mins } else { mins }))
}

fn digits(b: &[u8]) -> bool {
    b.iter().all(|c| c.is_ascii_digit())
}

fn num(b: &[u8]) -> u32 {
    b.iter().fold(0u32, |acc, c| acc * 10 + (c - b'0') as u32)
}

fn days_in_month(year: i32, month: u8) -> u8 {
    match month {
        1 | 3 | 5 | 7 | 8 | 10 | 12 => 31,
        4 | 6 | 9 | 11 => 30,
        2 => {
            if year % 4 == 0 && (year % 100 != 0 || year % 400 == 0) {
                29
            } else {
                28
            }
        }
        _ => 0,
    }
}

impl Datetime {
    /// The toml-test type tag for this datetime kind.
    pub fn type_tag(&self) -> &'static str {
        match self {
            Datetime::Offset { .. } => "datetime",
            Datetime::Local { .. } => "datetime-local",
            Datetime::DateOnly(_) => "date-local",
            Datetime::TimeOnly(_) => "time-local",
        }
    }
}

impl std::fmt::Display for Datetime {
    /// Renders in the same normalised form as the Go original: `T` as the
    /// separator, `Z` for a zero offset, and fractional seconds with trailing
    /// zeros trimmed (Go's `.999999999` layout verb).
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        match self {
            Datetime::Offset { date, time, offset } => {
                write!(f, "{}T{}{}", date, time, offset)
            }
            Datetime::Local { date, time } => write!(f, "{}T{}", date, time),
            Datetime::DateOnly(d) => write!(f, "{}", d),
            Datetime::TimeOnly(t) => write!(f, "{}", t),
        }
    }
}

impl std::fmt::Display for Date {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        write!(f, "{:04}-{:02}-{:02}", self.year, self.month, self.day)
    }
}

impl std::fmt::Display for Time {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        write!(f, "{:02}:{:02}:{:02}", self.hour, self.minute, self.second)?;
        if self.nanosecond != 0 {
            let frac = format!("{:09}", self.nanosecond);
            write!(f, ".{}", frac.trim_end_matches('0'))?;
        }
        Ok(())
    }
}

impl std::fmt::Display for TimeOffset {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        match self {
            TimeOffset::Z => write!(f, "Z"),
            TimeOffset::Offset(m) => {
                let (sign, m) = if *m < 0 { ('-', -*m) } else { ('+', *m) };
                write!(f, "{}{:02}:{:02}", sign, m / 60, m % 60)
            }
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    fn ok(s: &str) -> String {
        parse_datetime(s, 1, 1).expect("should parse").to_string()
    }
    fn err(s: &str) {
        assert!(parse_datetime(s, 1, 1).is_err(), "{:?} should have been rejected", s);
    }

    #[test]
    fn normalises_like_the_go_original() {
        assert_eq!(ok("1979-05-27t00:32:00z"), "1979-05-27T00:32:00Z");
        assert_eq!(ok("1979-05-27 00:32:00Z"), "1979-05-27T00:32:00Z");
        // A zero offset is the same instant as Z.
        assert_eq!(ok("1979-05-27T00:32:00-00:00"), "1979-05-27T00:32:00Z");
        // Trailing zeros in the fraction are trimmed.
        assert_eq!(ok("1987-07-05T17:45:56.600Z"), "1987-07-05T17:45:56.6Z");
        assert_eq!(ok("1979-05-27"), "1979-05-27");
        assert_eq!(ok("07:32:00"), "07:32:00");
        // Seconds are optional as of TOML 1.1.
        assert_eq!(ok("13:37"), "13:37:00");
    }

    #[test]
    fn rejects_malformed_datetimes() {
        err("1987-7-05T17:45:00Z");    // month needs a leading zero
        err("1987-07-0517:45:00Z");    // no date/time separator
        err("2023-10-01T1:32:00Z");    // hour needs a leading zero
        err("01:32:0");                // seconds need two digits
        err("24:00:00");               // hour out of range
        err("00:60:00");               // minute out of range
        err("00:00:61");               // second out of range
        err("12:13:14.");              // fraction needs a digit
        err("2006-01-30T");            // trailing separator
        err("1985-06-18 17:04:07+25:00"); // offset hour out of range
        err("2023-02-29");             // not a leap year
    }
}
