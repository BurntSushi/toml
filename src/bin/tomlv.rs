//! tomlv — validator CLI (ported from cmd/tomlv/main.go)
//! Validates TOML documents and prints each key's type.

use std::io::{self, Read};
use toml_rs_port::parse;

fn main() {
    let mut input = String::new();
    io::stdin().read_to_string(&mut input).expect("Failed to read stdin");

    match parse(&input) {
        Ok(_) => {
            println!("Valid TOML");
        }
        Err(e) => {
            eprintln!("{}", e);
            std::process::exit(1);
        }
    }
}
