package golitehtml

import (
	"bytes"
	"encoding/base64"
	"encoding/xml"
	"image"
	"image/draw"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"math"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/FloatTech/gg"
	builtfont "github.com/lianhong2758/golitehtml/font"
	"github.com/srwiley/oksvg"
	"github.com/srwiley/rasterx"
	xfont "golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
)

type imageLoader struct {
	baseDir string
	client  *http.Client

	mu    sync.Mutex
	cache map[string]image.Image
}

// newImageLoader 创建图片加载器；同一个 src 会缓存解码结果。
func newImageLoader(baseDir string) *imageLoader {
	return &imageLoader{
		baseDir: baseDir,
		client:  &http.Client{Timeout: 10 * time.Second},
		cache:   make(map[string]image.Image),
	}
}

// ImageSize 返回图片固有尺寸，布局阶段用它补全未指定的宽高。
func (l *imageLoader) ImageSize(src string) (Size, bool) {
	img, ok := l.Image(src)
	if !ok {
		return Size{}, false
	}
	bounds := img.Bounds()
	return Size{W: float64(bounds.Dx()), H: float64(bounds.Dy())}, true
}

// Image 加载并解码图片，支持本地路径、HTTP、data URL 和 SVG。
func (l *imageLoader) Image(src string) (image.Image, bool) {
	src = strings.TrimSpace(src)
	if src == "" {
		return nil, false
	}

	l.mu.Lock()
	if img, ok := l.cache[src]; ok {
		l.mu.Unlock()
		return img, true
	}
	l.mu.Unlock()

	img, ok := l.load(src)
	if !ok {
		return nil, false
	}

	l.mu.Lock()
	l.cache[src] = img
	l.mu.Unlock()
	return img, true
}

// load 读取原始图片字节，并按格式分派到标准图片解码或 SVG 栅格化。
func (l *imageLoader) load(src string) (image.Image, bool) {
	var data []byte
	var err error
	resolved := l.resolvePath(src)

	switch {
	case strings.HasPrefix(src, "data:"):
		data, err = decodeDataURL(src)
	case strings.HasPrefix(resolved, "http://") || strings.HasPrefix(resolved, "https://"):
		data, err = l.readHTTP(resolved)
	default:
		data, err = os.ReadFile(resolved)
	}
	if err != nil {
		return nil, false
	}

	if isSVG(src, resolved, data) {
		img, err := decodeSVG(data)
		return img, err == nil
	}

	img, _, err := image.Decode(bytes.NewReader(data))
	return img, err == nil
}

// readHTTP 下载远程图片资源。
func (l *imageLoader) readHTTP(src string) ([]byte, error) {
	resp, err := l.client.Get(src)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return io.ReadAll(resp.Body)
}

// resolvePath 将图片地址解析成可读取的本地路径或绝对 URL。
func (l *imageLoader) resolvePath(src string) string {
	if strings.HasPrefix(src, "file://") {
		if parsed, err := url.Parse(src); err == nil {
			return parsed.Path
		}
	}
	if strings.HasPrefix(l.baseDir, "http://") || strings.HasPrefix(l.baseDir, "https://") {
		base, err := url.Parse(l.baseDir)
		if err == nil {
			ref, err := url.Parse(src)
			if err == nil {
				return base.ResolveReference(ref).String()
			}
		}
	}
	if filepath.IsAbs(src) || l.baseDir == "" {
		return filepath.Clean(src)
	}
	return filepath.Join(l.baseDir, src)
}

// decodeDataURL 解码 data URL，支持 base64 和 utf8/url-escaped 两种常见形式。
func decodeDataURL(src string) ([]byte, error) {
	header, data, ok := strings.Cut(src, ",")
	if !ok {
		return nil, io.ErrUnexpectedEOF
	}
	if strings.Contains(header, ";base64") {
		return base64.StdEncoding.DecodeString(data)
	}
	decoded, err := url.PathUnescape(data)
	if err != nil {
		return nil, err
	}
	return []byte(decoded), nil
}

// isSVG 判断图片数据是否应按 SVG 处理。
func isSVG(src, resolved string, data []byte) bool {
	lowerSrc := strings.ToLower(src)
	lowerResolved := strings.ToLower(resolved)
	if strings.HasPrefix(lowerSrc, "data:image/svg+xml") {
		return true
	}
	if strings.HasSuffix(lowerResolved, ".svg") {
		return true
	}
	return bytes.HasPrefix(bytes.TrimSpace(data), []byte("<svg"))
}

// decodeSVG 使用 oksvg/rasterx 将 SVG 栅格化为 image.Image。
func decodeSVG(data []byte) (image.Image, error) {
	icon, err := oksvg.ReadIconStream(bytes.NewReader(data), oksvg.IgnoreErrorMode)
	if err != nil {
		return nil, err
	}

	width := int(math.Ceil(icon.ViewBox.W))
	height := int(math.Ceil(icon.ViewBox.H))
	if width <= 0 {
		width = 300
	}
	if height <= 0 {
		height = 150
	}

	dst := image.NewRGBA(image.Rect(0, 0, width, height))
	draw.Draw(dst, dst.Bounds(), image.Transparent, image.Point{}, draw.Src)
	icon.SetTarget(0, 0, float64(width), float64(height))
	scanner := rasterx.NewScannerGV(width, height, dst, dst.Bounds())
	raster := rasterx.NewDasher(width, height, scanner)
	icon.Draw(raster, 1)
	// oksvg 对 <text> 支持有限，这里补绘简单文本，满足示例和轻量图标场景。
	drawSVGText(dst, data)
	return dst, nil
}

type svgTextRun struct {
	x    float64
	y    float64
	size float64
	fill Color
	text string
}

func drawSVGText(dst *image.RGBA, data []byte) {
	runs := parseSVGTextRuns(data)
	if len(runs) == 0 {
		return
	}

	ttf, err := opentype.Parse(builtfont.TTF)
	if err != nil {
		return
	}
	dc := gg.NewContextForRGBA(dst)
	faces := make(map[string]xfont.Face, len(runs))
	for _, run := range runs {
		if strings.TrimSpace(run.text) == "" {
			continue
		}
		key := strconv.FormatFloat(run.size, 'f', 2, 64)
		face := faces[key]
		if face == nil {
			face, err = opentype.NewFace(ttf, &opentype.FaceOptions{
				Size:    run.size,
				DPI:     72,
				Hinting: xfont.HintingNone,
			})
			if err != nil {
				continue
			}
			faces[key] = face
		}
		dc.SetFontFace(face)
		dc.SetColor(toRGBA(run.fill))
		dc.DrawString(run.text, run.x, run.y)
	}
}

// parseSVGTextRuns 提取简单 <text> 节点的位置、字号、颜色和文本内容。
func parseSVGTextRuns(data []byte) []svgTextRun {
	decoder := xml.NewDecoder(bytes.NewReader(data))
	var runs []svgTextRun
	var current *svgTextRun

	for {
		token, err := decoder.Token()
		if err != nil {
			break
		}
		switch t := token.(type) {
		case xml.StartElement:
			if strings.EqualFold(t.Name.Local, "text") {
				run := svgTextRun{
					size: 16,
					fill: Color{R: 0, G: 0, B: 0, A: 255},
				}
				for _, attr := range t.Attr {
					switch strings.ToLower(attr.Name.Local) {
					case "x":
						run.x = parseSVGNumber(attr.Value, run.x)
					case "y":
						run.y = parseSVGNumber(attr.Value, run.y)
					case "font-size":
						run.size = parseSVGNumber(attr.Value, run.size)
					case "fill":
						if c, ok := ParseColor(attr.Value); ok {
							run.fill = c
						}
					}
				}
				current = &run
			}
		case xml.CharData:
			if current != nil {
				current.text += string(t)
			}
		case xml.EndElement:
			if current != nil && strings.EqualFold(t.Name.Local, "text") {
				runs = append(runs, *current)
				current = nil
			}
		}
	}
	return runs
}

// parseSVGNumber 解析 SVG 数值属性，当前只处理裸数字和 px。
func parseSVGNumber(value string, fallback float64) float64 {
	value = strings.TrimSpace(strings.TrimSuffix(value, "px"))
	if value == "" {
		return fallback
	}
	v, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return fallback
	}
	return v
}
