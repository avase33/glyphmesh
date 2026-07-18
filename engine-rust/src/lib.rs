//! glyphmesh engine — compiled to WebAssembly and run inside a browser Web
//! Worker, so heavy geometry never blocks the drawing thread.
//!
//! Two exports cross the Wasm boundary (see `proto/protocol.md`):
//!   - `simplify(points, epsilon)` — Ramer-Douglas-Peucker line simplification
//!   - `autocomplete(points)`      — recognise + snap the stroke to an ideal shape

mod geometry;
mod recognize;

use geometry::{rdp, to_flat, to_points};
use wasm_bindgen::prelude::*;

/// RDP polyline simplification over a flat `[x0,y0,x1,y1,...]` array.
#[wasm_bindgen]
pub fn simplify(points: Vec<f64>, epsilon: f64) -> Vec<f64> {
    let pts = to_points(&points);
    to_flat(&rdp(&pts, epsilon))
}

/// Recognise the stroke and return the idealised shape as a JSON string
/// (`{kind, confidence, points, closed}`).
#[wasm_bindgen]
pub fn autocomplete(points: Vec<f64>) -> String {
    serde_json::to_string(&recognize::recognize(&points)).unwrap_or_else(|_| "{}".into())
}

// Re-exports for native (`cargo test`) and rlib consumers.
pub use geometry::rdp as rdp_simplify;
pub use recognize::{recognize, Recognition};

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn simplify_roundtrips_a_line() {
        let flat = vec![0.0, 0.0, 1.0, 0.1, 2.0, 0.0, 3.0, 0.0];
        let out = simplify(flat, 0.5);
        assert_eq!(out, vec![0.0, 0.0, 3.0, 0.0]);
    }

    #[test]
    fn autocomplete_returns_json() {
        let flat = vec![0.0, 0.0, 10.0, 0.0, 20.0, 0.0, 30.0, 0.0];
        let json = autocomplete(flat);
        assert!(json.contains("\"kind\""));
        assert!(json.contains("line"));
    }
}
