//! Metadata — ported from meta.go (145 LOC)
//! Tracks metadata about decoded keys (whether defined, type info).

use std::collections::HashSet;

#[derive(Debug, Clone, Default)]
pub struct Metadata {
    pub keys: Vec<String>,
    pub defined: HashSet<String>,
}

impl Metadata {
    pub fn is_defined(&self, key: &str) -> bool {
        self.defined.contains(key)
    }
}
