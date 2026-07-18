//! Pure geometry: point type, Ramer-Douglas-Peucker simplification, and the
//! primitive measurements the recogniser is built on. No wasm here — this all
//! compiles and tests natively.

#[derive(Clone, Copy, Debug, PartialEq)]
pub struct Point {
    pub x: f64,
    pub y: f64,
}

impl Point {
    pub fn new(x: f64, y: f64) -> Self {
        Point { x, y }
    }
    pub fn dist(&self, o: &Point) -> f64 {
        ((self.x - o.x).powi(2) + (self.y - o.y).powi(2)).sqrt()
    }
}

/// Flatten `[x0,y0,x1,y1,...]` into points; a trailing odd value is ignored.
pub fn to_points(flat: &[f64]) -> Vec<Point> {
    flat.chunks_exact(2).map(|c| Point::new(c[0], c[1])).collect()
}

/// Flatten points back to `[x0,y0,...]`.
pub fn to_flat(pts: &[Point]) -> Vec<f64> {
    let mut out = Vec::with_capacity(pts.len() * 2);
    for p in pts {
        out.push(p.x);
        out.push(p.y);
    }
    out
}

/// Perpendicular distance from `p` to the line through `a`-`b`.
fn perp_distance(p: &Point, a: &Point, b: &Point) -> f64 {
    let dx = b.x - a.x;
    let dy = b.y - a.y;
    let len = (dx * dx + dy * dy).sqrt();
    if len < 1e-12 {
        return p.dist(a);
    }
    // |cross product| / |ab|
    ((p.x - a.x) * dy - (p.y - a.y) * dx).abs() / len
}

/// Ramer-Douglas-Peucker polyline simplification.
pub fn rdp(points: &[Point], epsilon: f64) -> Vec<Point> {
    if points.len() < 3 {
        return points.to_vec();
    }
    let a = points[0];
    let b = points[points.len() - 1];
    let mut max_d = 0.0;
    let mut idx = 0;
    for (i, p) in points.iter().enumerate().take(points.len() - 1).skip(1) {
        let d = perp_distance(p, &a, &b);
        if d > max_d {
            max_d = d;
            idx = i;
        }
    }
    if max_d > epsilon {
        let mut left = rdp(&points[..=idx], epsilon);
        let right = rdp(&points[idx..], epsilon);
        left.pop(); // shared middle point
        left.extend(right);
        left
    } else {
        vec![a, b]
    }
}

/// Total path length of a polyline.
pub fn path_length(pts: &[Point]) -> f64 {
    pts.windows(2).map(|w| w[0].dist(&w[1])).sum()
}

pub struct BBox {
    pub min_x: f64,
    pub min_y: f64,
    pub max_x: f64,
    pub max_y: f64,
}

impl BBox {
    pub fn of(pts: &[Point]) -> BBox {
        let mut b = BBox {
            min_x: f64::INFINITY,
            min_y: f64::INFINITY,
            max_x: f64::NEG_INFINITY,
            max_y: f64::NEG_INFINITY,
        };
        for p in pts {
            b.min_x = b.min_x.min(p.x);
            b.min_y = b.min_y.min(p.y);
            b.max_x = b.max_x.max(p.x);
            b.max_y = b.max_y.max(p.y);
        }
        b
    }
    pub fn width(&self) -> f64 {
        self.max_x - self.min_x
    }
    pub fn height(&self) -> f64 {
        self.max_y - self.min_y
    }
    pub fn center(&self) -> Point {
        Point::new((self.min_x + self.max_x) / 2.0, (self.min_y + self.max_y) / 2.0)
    }
}

/// Turning angle (radians) at vertex `b` given neighbours `a`,`c`.
pub fn turn_angle(a: &Point, b: &Point, c: &Point) -> f64 {
    let v1 = (a.x - b.x, a.y - b.y);
    let v2 = (c.x - b.x, c.y - b.y);
    let dot = v1.0 * v2.0 + v1.1 * v2.1;
    let m1 = (v1.0 * v1.0 + v1.1 * v1.1).sqrt();
    let m2 = (v2.0 * v2.0 + v2.1 * v2.1).sqrt();
    if m1 < 1e-9 || m2 < 1e-9 {
        return std::f64::consts::PI;
    }
    (dot / (m1 * m2)).clamp(-1.0, 1.0).acos()
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn rdp_collapses_straight_line() {
        let pts = vec![
            Point::new(0.0, 0.0),
            Point::new(1.0, 0.05),
            Point::new(2.0, -0.03),
            Point::new(3.0, 0.02),
            Point::new(4.0, 0.0),
        ];
        let out = rdp(&pts, 0.5);
        assert_eq!(out.len(), 2);
        assert_eq!(out[0], Point::new(0.0, 0.0));
        assert_eq!(out[1], Point::new(4.0, 0.0));
    }

    #[test]
    fn rdp_keeps_a_corner() {
        let pts = vec![
            Point::new(0.0, 0.0),
            Point::new(5.0, 0.0),
            Point::new(10.0, 0.0),
            Point::new(10.0, 5.0),
            Point::new(10.0, 10.0),
        ];
        let out = rdp(&pts, 0.5);
        // the (10,0) corner must survive
        assert!(out.iter().any(|p| *p == Point::new(10.0, 0.0)));
    }

    #[test]
    fn bbox_and_length() {
        let pts = vec![Point::new(0.0, 0.0), Point::new(3.0, 4.0)];
        assert!((path_length(&pts) - 5.0).abs() < 1e-9);
        let b = BBox::of(&pts);
        assert_eq!(b.width(), 3.0);
        assert_eq!(b.height(), 4.0);
    }
}
