//! Shared toml-test comparison logic, used by both the conformance runner
//! and the differential fuzzer.
//!
//! Mirrors `internal/toml-test/json.go`: type tags must match exactly, floats
//! are compared numerically, datetimes as instants, booleans case-insensitively,
//! everything else as strings.

#![allow(dead_code)]

use serde_json::{json, Value as JsonValue};
use toml_rs_port::Value;

pub fn value_to_json(value: &Value) -> JsonValue {
    match value {
        Value::Array(arr) => JsonValue::Array(arr.iter().map(value_to_json).collect()),
        Value::Table(table) => {
            let mut map = serde_json::Map::new();
            for (k, v) in table.iter() { map.insert(k.clone(), value_to_json(v)); }
            JsonValue::Object(map)
        }
        scalar => json!({
            "type": scalar.type_tag().expect("scalars always carry a type tag"),
            "value": scalar.value_string().expect("scalars always render a value"),
        }),
    }
}

pub fn compare(want: &JsonValue, have: &JsonValue) -> Result<(), String> {
    match (want, have) {
        (JsonValue::Object(w), JsonValue::Object(h)) => {
            // A two-field {type, value} object is a tagged scalar; anything
            // else is a table.
            if is_tagged(w) || is_tagged(h) {
                if !(is_tagged(w) && is_tagged(h)) {
                    return Err(format!("one side is a scalar, the other a table: {} vs {}", want, have));
                }
                return compare_scalar(w, h);
            }
            for k in w.keys() {
                if !h.contains_key(k) { return Err(format!("missing key {:?}", k)); }
            }
            for k in h.keys() {
                if !w.contains_key(k) { return Err(format!("unexpected key {:?}", k)); }
            }
            for (k, wv) in w {
                compare(wv, &h[k]).map_err(|e| format!("{}: {}", k, e))?;
            }
            Ok(())
        }
        (JsonValue::Array(w), JsonValue::Array(h)) => {
            if w.len() != h.len() {
                return Err(format!("array length {} != {}", w.len(), h.len()));
            }
            for (i, (wv, hv)) in w.iter().zip(h).enumerate() {
                compare(wv, hv).map_err(|e| format!("[{}]: {}", i, e))?;
            }
            Ok(())
        }
        _ => Err(format!("shape mismatch: {} vs {}", want, have)),
    }
}

pub fn is_tagged(o: &serde_json::Map<String, JsonValue>) -> bool {
    o.len() == 2 && o.contains_key("type") && o.contains_key("value")
}

pub fn compare_scalar(
    w: &serde_json::Map<String, JsonValue>,
    h: &serde_json::Map<String, JsonValue>,
) -> Result<(), String> {
    let (wt, ht) = (w["type"].as_str().unwrap_or(""), h["type"].as_str().unwrap_or(""));
    if wt != ht {
        return Err(format!("type {:?} != {:?}", wt, ht));
    }
    let (wv, hv) = (w["value"].as_str().unwrap_or(""), h["value"].as_str().unwrap_or(""));
    match wt {
        "float" => compare_floats(wv, hv),
        "datetime" | "datetime-local" | "date-local" | "time-local" => compare_datetimes(wt, wv, hv),
        "bool" => {
            if wv.eq_ignore_ascii_case(hv) { Ok(()) } else { Err(format!("{:?} != {:?}", wv, hv)) }
        }
        _ => {
            if wv == hv { Ok(()) } else { Err(format!("{:?} != {:?}", wv, hv)) }
        }
    }
}

pub fn compare_floats(want: &str, have: &str) -> Result<(), String> {
    let (w, h) = (want.to_ascii_lowercase(), have.to_ascii_lowercase());
    if w.ends_with("nan") || h.ends_with("nan") {
        // NaN != NaN, so compare the spelling with any sign stripped.
        let (w, h) = (w.trim_start_matches(['-', '+']), h.trim_start_matches(['-', '+']));
        return if w == h { Ok(()) } else { Err(format!("{:?} != {:?}", w, h)) };
    }
    let wf: f64 = w.parse().map_err(|_| format!("expected value {:?} is not a float", want))?;
    let hf: f64 = h.parse().map_err(|_| format!("produced value {:?} is not a float", have))?;
    if wf == hf { Ok(()) } else { Err(format!("{} != {}", wf, hf)) }
}

/// Normalise to a comparable instant: `T` separator, uppercase `Z`, zero
/// offsets folded to `Z`, and fractional seconds padded to nine digits.
pub fn normalize_datetime(kind: &str, s: &str) -> Option<String> {
    let mut s = s.replace(' ', "T").replace('t', "T").replace('z', "Z");

    // Split off the offset, if any, so it can be normalised on its own.
    let mut offset = String::new();
    if kind == "datetime" {
        if let Some(rest) = s.strip_suffix('Z') {
            offset = "+00:00".into();
            s = rest.to_string();
        } else {
            let idx = s.rfind(['+', '-']).filter(|i| *i > 7)?;
            let off = &s[idx..];
            offset = if off == "-00:00" { "+00:00".into() } else { off.to_string() };
            s = s[..idx].to_string();
        }
    }

    // Pad fractional seconds so `.6` and `.600` normalise identically.
    if let Some(dot) = s.find('.') {
        let frac: String = s[dot + 1..].chars().take_while(|c| c.is_ascii_digit()).collect();
        let tail = &s[dot + 1 + frac.len()..];
        s = format!("{}.{:0<9}{}", &s[..dot], &frac[..frac.len().min(9)], tail);
    } else {
        s.push_str(".000000000");
    }

    Some(format!("{}{}", s, offset))
}

pub fn compare_datetimes(kind: &str, want: &str, have: &str) -> Result<(), String> {
    let w = normalize_datetime(kind, want).ok_or_else(|| format!("expected value {:?} is not a {}", want, kind))?;
    let h = normalize_datetime(kind, have).ok_or_else(|| format!("produced value {:?} is not a {}", have, kind))?;
    if w == h { Ok(()) } else { Err(format!("{:?} != {:?}", want, have)) }
}

