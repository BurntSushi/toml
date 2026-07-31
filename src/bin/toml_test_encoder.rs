//! toml-test-encoder — implements the toml-test encoder protocol.
//! Accepts JSON on stdin, outputs TOML on stdout.

use std::io::{self, Read};
use toml_rs_port::{Value, encode};

fn main() {
    let mut input = String::new();
    io::stdin().read_to_string(&mut input).expect("Failed to read stdin");

    // TODO: parse JSON input into Value tree, then encode to TOML
    match encode(&Value::String(input)) {
        Ok(toml) => print!("{}", toml),
        Err(e) => {
            eprintln!("{}", e);
            std::process::exit(1);
        }
    }
}
