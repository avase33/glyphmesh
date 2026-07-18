//! Shape recognition + autocomplete.
//!
//! Given a raw stroke, we simplify it (RDP), measure a few geometric
//! invariants — is it closed? how round? how many corners? — and snap it to the
//! nearest ideal primitive: line, rectangle, triangle, circle, arrow, or (if
//! nothing fits) the cleaned freeform stroke. This is the "AI autocompletes
//! your drawing" step, done with honest geometry rather than a black box.

use crate::geometry::{path_length, rdp, to_flat, to_points, turn_angle, BBox, Point};
use serde::Serialize;
use std::f64::consts::PI;

#[derive(Serialize, Debug)]
pub struct Recognition {
    pub kind: String,
    pub confidence: f64,
    pub points: Vec<f64>,
    pub closed: bool,
}

const CORNER_MAX_ANGLE: f64 = 2.4; // radians; below this a vertex is a "corner"

fn regular_circle(center: Point, radius: f64, n: usize) -> Vec<f64> {
    let mut out = Vec::with_capacity(n * 2);
    for i in 0..n {
        let a = 2.0 * PI * (i as f64) / (n as f64);
        out.push(center.x + radius * a.cos());
        out.push(center.y + radius * a.sin());
    }
    out
}

fn radius_stats(pts: &[Point], center: Point) -> (f64, f64) {
    let n = pts.len() as f64;
    if n == 0.0 {
        return (0.0, 0.0);
    }
    let mean = pts.iter().map(|p| p.dist(&center)).sum::<f64>() / n;
    let var = pts.iter().map(|p| (p.dist(&center) - mean).powi(2)).sum::<f64>() / n;
    (mean, var.sqrt())
}

/// Corner vertices of a cyclic polygon and their angles.
fn cyclic_corner_angles(verts: &[Point]) -> Vec<f64> {
    let n = verts.len();
    let mut angles = Vec::new();
    for i in 0..n {
        let a = verts[(i + n - 1) % n];
        let b = verts[i];
        let c = verts[(i + 1) % n];
        angles.push(turn_angle(&a, &b, &c));
    }
    angles
}

pub fn recognize(flat: &[f64]) -> Recognition {
    let pts = to_points(flat);
    if pts.len() < 2 {
        return Recognition {
            kind: "freeform".into(),
            confidence: 0.0,
            points: flat.to_vec(),
            closed: false,
        };
    }

    let bbox = BBox::of(&pts);
    let diag = (bbox.width().powi(2) + bbox.height().powi(2)).sqrt().max(1.0);
    let eps = (diag * 0.03).max(2.0);
    let simp = rdp(&pts, eps);
    let closed = pts.len() > 2 && pts[0].dist(&pts[pts.len() - 1]) < diag * 0.2;

    if closed {
        // circle: points sit at a near-constant radius from the centre
        let center = bbox.center();
        let (mean_r, std_r) = radius_stats(&pts, center);
        let roundness = if mean_r > 1e-6 { std_r / mean_r } else { 1.0 };
        if roundness < 0.22 && simp.len() >= 5 {
            return Recognition {
                kind: "circle".into(),
                confidence: (1.0 - roundness / 0.22).clamp(0.5, 0.99),
                points: regular_circle(center, mean_r, 24),
                closed: true,
            };
        }

        // polygon: drop the duplicated closing vertex, count corners
        let mut verts = simp.clone();
        if verts.len() >= 2 && verts[0].dist(&verts[verts.len() - 1]) < eps * 1.5 {
            verts.pop();
        }
        let angles = cyclic_corner_angles(&verts);
        let corners: Vec<usize> = angles
            .iter()
            .enumerate()
            .filter(|(_, a)| **a < CORNER_MAX_ANGLE)
            .map(|(i, _)| i)
            .collect();

        if corners.len() == 4 {
            let dev: f64 = corners
                .iter()
                .map(|&i| (angles[i] - PI / 2.0).abs())
                .sum::<f64>()
                / 4.0;
            let rect = vec![
                bbox.min_x, bbox.min_y, bbox.max_x, bbox.min_y, bbox.max_x, bbox.max_y,
                bbox.min_x, bbox.max_y,
            ];
            return Recognition {
                kind: "rectangle".into(),
                confidence: (1.0 - dev / (PI / 4.0)).clamp(0.5, 0.99),
                points: rect,
                closed: true,
            };
        }

        if corners.len() == 3 {
            let tri: Vec<f64> = corners.iter().flat_map(|&i| [verts[i].x, verts[i].y]).collect();
            return Recognition {
                kind: "triangle".into(),
                confidence: 0.8,
                points: tri,
                closed: true,
            };
        }

        return Recognition {
            kind: "freeform".into(),
            confidence: 0.4,
            points: to_flat(&simp),
            closed: true,
        };
    }

    // open strokes ---------------------------------------------------------
    // straight line: the whole stroke barely deviates from its chord
    if simp.len() == 2 {
        return Recognition {
            kind: "line".into(),
            confidence: 0.95,
            points: vec![pts[0].x, pts[0].y, pts[pts.len() - 1].x, pts[pts.len() - 1].y],
            closed: false,
        };
    }

    if let Some(arrow) = detect_arrow(&simp) {
        return arrow;
    }

    // nearly straight (few interior points, small deviation) -> line
    let chord = pts[0].dist(&pts[pts.len() - 1]);
    if chord > 1e-6 && path_length(&pts) / chord < 1.10 {
        return Recognition {
            kind: "line".into(),
            confidence: 0.85,
            points: vec![pts[0].x, pts[0].y, pts[pts.len() - 1].x, pts[pts.len() - 1].y],
            closed: false,
        };
    }

    Recognition {
        kind: "freeform".into(),
        confidence: 0.5,
        points: to_flat(&simp),
        closed: false,
    }
}

/// An arrow is a dominant shaft segment ending in two short "barb" segments
/// that fold back from the tip.
fn detect_arrow(simp: &[Point]) -> Option<Recognition> {
    if simp.len() < 4 || simp.len() > 6 {
        return None;
    }
    let seg_lens: Vec<f64> = simp.windows(2).map(|w| w[0].dist(&w[1])).collect();
    let total: f64 = seg_lens.iter().sum();
    if total < 1e-6 {
        return None;
    }
    // the first segment should dominate (the shaft)
    let shaft = seg_lens[0];
    if shaft / total < 0.5 {
        return None;
    }
    // the two final segments (barbs) should be short and fold back toward the shaft
    let tip = simp[1]; // end of the shaft
    let barb_end = simp[simp.len() - 1];
    let shaft_dir = (tip.x - simp[0].x, tip.y - simp[0].y);
    let barb_dir = (barb_end.x - tip.x, barb_end.y - tip.y);
    let dot = shaft_dir.0 * barb_dir.0 + shaft_dir.1 * barb_dir.1;
    // barb should point somewhat backward relative to the shaft
    if dot >= 0.0 {
        return None;
    }
    let flat: Vec<f64> = simp.iter().flat_map(|p| [p.x, p.y]).collect();
    Some(Recognition {
        kind: "arrow".into(),
        confidence: 0.7,
        points: flat,
        closed: false,
    })
}

#[cfg(test)]
mod tests {
    use super::*;

    fn stroke(pairs: &[(f64, f64)]) -> Vec<f64> {
        pairs.iter().flat_map(|&(x, y)| [x, y]).collect()
    }

    #[test]
    fn recognizes_line() {
        let s = stroke(&[(0.0, 0.0), (10.0, 1.0), (20.0, 0.5), (30.0, 0.0)]);
        let r = recognize(&s);
        assert_eq!(r.kind, "line");
        assert_eq!(r.points.len(), 4);
    }

    #[test]
    fn recognizes_rectangle() {
        // a closed, slightly wobbly square
        let s = stroke(&[
            (0.0, 0.0), (50.0, 2.0), (100.0, 0.0), (101.0, 50.0), (100.0, 100.0),
            (50.0, 99.0), (0.0, 100.0), (1.0, 50.0), (0.0, 1.0),
        ]);
        let r = recognize(&s);
        assert_eq!(r.kind, "rectangle");
        assert!(r.closed);
        assert_eq!(r.points.len(), 8);
    }

    #[test]
    fn recognizes_circle() {
        // sampled unit-ish circle, closed
        let mut pts = Vec::new();
        let n = 40;
        for i in 0..=n {
            let a = 2.0 * PI * (i as f64) / (n as f64);
            pts.push((100.0 + 50.0 * a.cos(), 100.0 + 50.0 * a.sin()));
        }
        let r = recognize(&stroke(&pts));
        assert_eq!(r.kind, "circle");
        assert!(r.points.len() >= 10);
    }

    #[test]
    fn recognizes_triangle() {
        let s = stroke(&[
            (0.0, 0.0), (50.0, 0.0), (100.0, 0.0), (50.0, 86.0), (0.0, 1.0),
        ]);
        let r = recognize(&s);
        assert_eq!(r.kind, "triangle");
        assert_eq!(r.points.len(), 6);
    }
}
