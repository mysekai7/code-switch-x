package main

import (
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
	"path/filepath"
)

const (
	canvasSize  = 1024
	sampleScale = 4
)

type paint struct {
	r float64
	g float64
	b float64
	a float64
}

func main() {
	img := makeIcon(canvasSize)

	for _, path := range []string{
		"build/appicon.png",
		"assets/icon.png",
		"assets/icon-dark.png",
	} {
		if err := writePNG(path, img); err != nil {
			panic(err)
		}
	}
}

func makeIcon(size int) *image.RGBA {
	hiSize := size * sampleScale
	hi := image.NewRGBA(image.Rect(0, 0, hiSize, hiSize))

	drawBackground(hi, size)
	drawRoute(hi, 252, 252, 772, 772, paint{0.13, 0.83, 0.93, 1})
	drawRoute(hi, 772, 252, 252, 772, paint{0.20, 0.83, 0.60, 1})

	drawEndpoint(hi, 252, 252, paint{0.13, 0.83, 0.93, 1})
	drawEndpoint(hi, 772, 772, paint{0.13, 0.83, 0.93, 1})
	drawEndpoint(hi, 772, 252, paint{0.20, 0.83, 0.60, 1})
	drawEndpoint(hi, 252, 772, paint{0.20, 0.83, 0.60, 1})
	drawHub(hi)

	return downsample(hi, size)
}

func drawBackground(img *image.RGBA, size int) {
	hiSize := size * sampleScale
	inset := 44.0
	radius := 214.0

	for py := 0; py < hiSize; py++ {
		y := (float64(py) + 0.5) / sampleScale
		for px := 0; px < hiSize; px++ {
			x := (float64(px) + 0.5) / sampleScale
			if !insideRoundedRect(x, y, inset, inset, float64(size)-inset, float64(size)-inset, radius) {
				continue
			}

			t := (x + y) / float64(size*2)
			bg := mix(paint{0.059, 0.090, 0.165, 1}, paint{0.067, 0.094, 0.153, 1}, t)
			bg = addRadial(bg, x, y, 770, 235, 520, paint{0.13, 0.83, 0.93, 1}, 0.22)
			bg = addRadial(bg, x, y, 255, 785, 540, paint{0.20, 0.83, 0.60, 1}, 0.18)
			bg = addRadial(bg, x, y, 512, 512, 610, paint{0.90, 0.98, 1.00, 1}, 0.06)

			edge := distanceToRoundedRectEdge(x, y, inset, inset, float64(size)-inset, float64(size)-inset, radius)
			if edge < 3 {
				bg = mix(bg, paint{0.90, 0.98, 1.00, 1}, (3-edge)*0.10)
			}

			img.SetRGBA(px, py, toRGBA(bg))
		}
	}
}

func drawRoute(img *image.RGBA, x1, y1, x2, y2 float64, c paint) {
	drawCapsule(img, x1, y1, x2, y2, 120, paint{c.r, c.g, c.b, 0.08})
	drawCapsule(img, x1, y1, x2, y2, 76, paint{c.r, c.g, c.b, 0.18})
	drawCapsule(img, x1, y1, x2, y2, 42, paint{0.02, 0.04, 0.07, 0.56})
	drawCapsule(img, x1, y1, x2, y2, 28, paint{c.r, c.g, c.b, 0.86})
	drawCapsule(img, x1, y1, x2, y2, 9, paint{0.90, 0.98, 1.00, 0.76})
}

func drawEndpoint(img *image.RGBA, x, y float64, c paint) {
	drawCircle(img, x, y, 58, paint{c.r, c.g, c.b, 0.10})
	drawCircle(img, x, y, 42, paint{c.r, c.g, c.b, 0.62})
	drawCircle(img, x, y, 34, paint{0.03, 0.06, 0.11, 0.98})
	drawCircle(img, x, y, 18, paint{c.r, c.g, c.b, 0.24})
	drawCircle(img, x, y, 10, paint{0.90, 0.98, 1.00, 0.92})
}

func drawHub(img *image.RGBA) {
	drawCircle(img, 512, 512, 132, paint{0.13, 0.83, 0.93, 0.10})
	drawCircle(img, 512, 512, 106, paint{0.20, 0.83, 0.60, 0.10})
	drawCircle(img, 512, 512, 86, paint{0.13, 0.83, 0.93, 0.46})
	drawCircle(img, 512, 512, 78, paint{0.03, 0.06, 0.11, 0.97})
	drawCircle(img, 512, 512, 46, paint{0.90, 0.98, 1.00, 0.07})

	drawCapsule(img, 480, 480, 544, 544, 14, paint{0.13, 0.83, 0.93, 0.82})
	drawCapsule(img, 544, 480, 480, 544, 14, paint{0.20, 0.83, 0.60, 0.82})
	drawCircle(img, 512, 512, 13, paint{0.90, 0.98, 1.00, 0.94})
}

func drawCapsule(img *image.RGBA, x1, y1, x2, y2, width float64, c paint) {
	pad := width/2 + 3
	minX := int(math.Floor(math.Min(x1, x2)-pad) * sampleScale)
	maxX := int(math.Ceil(math.Max(x1, x2)+pad) * sampleScale)
	minY := int(math.Floor(math.Min(y1, y2)-pad) * sampleScale)
	maxY := int(math.Ceil(math.Max(y1, y2)+pad) * sampleScale)
	minX, minY = max(0, minX), max(0, minY)
	maxX, maxY = min(img.Bounds().Dx(), maxX), min(img.Bounds().Dy(), maxY)

	radius := width / 2
	for py := minY; py < maxY; py++ {
		y := (float64(py) + 0.5) / sampleScale
		for px := minX; px < maxX; px++ {
			x := (float64(px) + 0.5) / sampleScale
			d := distanceToSegment(x, y, x1, y1, x2, y2)
			if d > radius+1.5 {
				continue
			}
			alpha := c.a * (1 - smoothstep(radius-1.5, radius+1.5, d))
			blend(img, px, py, paint{c.r, c.g, c.b, alpha})
		}
	}
}

func drawCircle(img *image.RGBA, cx, cy, radius float64, c paint) {
	pad := radius + 3
	minX := max(0, int(math.Floor(cx-pad))*sampleScale)
	maxX := min(img.Bounds().Dx(), int(math.Ceil(cx+pad))*sampleScale)
	minY := max(0, int(math.Floor(cy-pad))*sampleScale)
	maxY := min(img.Bounds().Dy(), int(math.Ceil(cy+pad))*sampleScale)

	for py := minY; py < maxY; py++ {
		y := (float64(py) + 0.5) / sampleScale
		for px := minX; px < maxX; px++ {
			x := (float64(px) + 0.5) / sampleScale
			d := math.Hypot(x-cx, y-cy)
			if d > radius+1.5 {
				continue
			}
			alpha := c.a * (1 - smoothstep(radius-1.5, radius+1.5, d))
			blend(img, px, py, paint{c.r, c.g, c.b, alpha})
		}
	}
}

func insideRoundedRect(x, y, left, top, right, bottom, radius float64) bool {
	if x < left || x > right || y < top || y > bottom {
		return false
	}

	cx := clamp(x, left+radius, right-radius)
	cy := clamp(y, top+radius, bottom-radius)
	return math.Hypot(x-cx, y-cy) <= radius
}

func distanceToRoundedRectEdge(x, y, left, top, right, bottom, radius float64) float64 {
	straightEdge := math.Min(math.Min(x-left, right-x), math.Min(y-top, bottom-y))
	cx := clamp(x, left+radius, right-radius)
	cy := clamp(y, top+radius, bottom-radius)
	cornerEdge := radius - math.Hypot(x-cx, y-cy)
	return math.Min(straightEdge, cornerEdge)
}

func distanceToSegment(px, py, x1, y1, x2, y2 float64) float64 {
	dx := x2 - x1
	dy := y2 - y1
	if dx == 0 && dy == 0 {
		return math.Hypot(px-x1, py-y1)
	}

	t := ((px-x1)*dx + (py-y1)*dy) / (dx*dx + dy*dy)
	t = clamp(t, 0, 1)
	x := x1 + t*dx
	y := y1 + t*dy
	return math.Hypot(px-x, py-y)
}

func addRadial(base paint, x, y, cx, cy, radius float64, c paint, strength float64) paint {
	d := math.Hypot(x-cx, y-cy)
	if d >= radius {
		return base
	}
	t := 1 - d/radius
	t = t * t * strength
	base.r = clamp(base.r+c.r*t, 0, 1)
	base.g = clamp(base.g+c.g*t, 0, 1)
	base.b = clamp(base.b+c.b*t, 0, 1)
	return base
}

func mix(a, b paint, t float64) paint {
	t = clamp(t, 0, 1)
	return paint{
		r: a.r + (b.r-a.r)*t,
		g: a.g + (b.g-a.g)*t,
		b: a.b + (b.b-a.b)*t,
		a: a.a + (b.a-a.a)*t,
	}
}

func smoothstep(edge0, edge1, x float64) float64 {
	t := clamp((x-edge0)/(edge1-edge0), 0, 1)
	return t * t * (3 - 2*t)
}

func blend(img *image.RGBA, x, y int, src paint) {
	if src.a <= 0 {
		return
	}

	dst := img.RGBAAt(x, y)
	da := float64(dst.A) / 255
	sa := clamp(src.a, 0, 1)
	outA := sa + da*(1-sa)
	if outA == 0 {
		img.SetRGBA(x, y, color.RGBA{})
		return
	}

	dr := float64(dst.R) / 255
	dg := float64(dst.G) / 255
	db := float64(dst.B) / 255
	outR := (src.r*sa + dr*da*(1-sa)) / outA
	outG := (src.g*sa + dg*da*(1-sa)) / outA
	outB := (src.b*sa + db*da*(1-sa)) / outA

	img.SetRGBA(x, y, color.RGBA{
		R: byte(clamp(outR, 0, 1)*255 + 0.5),
		G: byte(clamp(outG, 0, 1)*255 + 0.5),
		B: byte(clamp(outB, 0, 1)*255 + 0.5),
		A: byte(clamp(outA, 0, 1)*255 + 0.5),
	})
}

func downsample(src *image.RGBA, size int) *image.RGBA {
	dst := image.NewRGBA(image.Rect(0, 0, size, size))
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			var r, g, b, a int
			for sy := 0; sy < sampleScale; sy++ {
				for sx := 0; sx < sampleScale; sx++ {
					c := src.RGBAAt(x*sampleScale+sx, y*sampleScale+sy)
					r += int(c.R)
					g += int(c.G)
					b += int(c.B)
					a += int(c.A)
				}
			}
			n := sampleScale * sampleScale
			dst.SetRGBA(x, y, color.RGBA{
				R: byte(r / n),
				G: byte(g / n),
				B: byte(b / n),
				A: byte(a / n),
			})
		}
	}
	return dst
}

func writePNG(path string, img image.Image) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	encoder := png.Encoder{CompressionLevel: png.BestCompression}
	return encoder.Encode(file, img)
}

func toRGBA(c paint) color.RGBA {
	return color.RGBA{
		R: byte(clamp(c.r, 0, 1)*255 + 0.5),
		G: byte(clamp(c.g, 0, 1)*255 + 0.5),
		B: byte(clamp(c.b, 0, 1)*255 + 0.5),
		A: byte(clamp(c.a, 0, 1)*255 + 0.5),
	}
}

func clamp(v, lo, hi float64) float64 {
	return math.Max(lo, math.Min(hi, v))
}
