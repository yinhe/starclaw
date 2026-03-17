package main

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/jpeg"
	"log"
	"os"
)

func main() {
	if len(os.Args) < 3 {
		log.Fatal("usage: jpg2ico input.jpg output.ico")
	}

	f, err := os.Open(os.Args[1])
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()

	src, err := jpeg.Decode(f)
	if err != nil {
		log.Fatal(err)
	}

	// Generate multiple sizes for ICO
	sizes := []int{256, 64, 48, 32, 16}
	type entry struct {
		size int
		data []byte
	}
	var entries []entry

	for _, sz := range sizes {
		resized := resize(src, sz)
		var buf bytes.Buffer
		// Write as BMP (DIB) without file header
		if err := writeDIB(&buf, resized); err != nil {
			log.Fatal(err)
		}
		entries = append(entries, entry{size: sz, data: buf.Bytes()})
	}

	// Write ICO file
	out, err := os.Create(os.Args[2])
	if err != nil {
		log.Fatal(err)
	}
	defer out.Close()

	// ICO header: reserved(2) + type(2) + count(2)
	binary.Write(out, binary.LittleEndian, uint16(0))            // reserved
	binary.Write(out, binary.LittleEndian, uint16(1))            // type: icon
	binary.Write(out, binary.LittleEndian, uint16(len(entries))) // count

	// Calculate offsets
	headerSize := 6 + len(entries)*16
	offset := headerSize
	for _, e := range entries {
		w := byte(e.size)
		h := byte(e.size)
		if e.size == 256 {
			w = 0 // 0 means 256
			h = 0
		}
		binary.Write(out, binary.LittleEndian, w)                   // width
		binary.Write(out, binary.LittleEndian, h)                   // height
		binary.Write(out, binary.LittleEndian, byte(0))             // color palette
		binary.Write(out, binary.LittleEndian, byte(0))             // reserved
		binary.Write(out, binary.LittleEndian, uint16(1))           // color planes
		binary.Write(out, binary.LittleEndian, uint16(32))          // bits per pixel
		binary.Write(out, binary.LittleEndian, uint32(len(e.data))) // size
		binary.Write(out, binary.LittleEndian, uint32(offset))      // offset
		offset += len(e.data)
	}

	for _, e := range entries {
		out.Write(e.data)
	}

	log.Printf("Created %s with %d sizes", os.Args[2], len(entries))
}

// resize using nearest-neighbor (no external deps needed)
func resize(src image.Image, size int) *image.RGBA {
	dst := image.NewRGBA(image.Rect(0, 0, size, size))
	srcBounds := src.Bounds()
	srcW := srcBounds.Dx()
	srcH := srcBounds.Dy()

	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			srcX := srcBounds.Min.X + x*srcW/size
			srcY := srcBounds.Min.Y + y*srcH/size
			dst.Set(x, y, src.At(srcX, srcY))
		}
	}
	return dst
}

// writeDIB writes a BMP DIB (without file header) suitable for ICO embedding
func writeDIB(buf *bytes.Buffer, img *image.RGBA) error {
	bounds := img.Bounds()
	w := bounds.Dx()
	h := bounds.Dy()

	// BITMAPINFOHEADER (40 bytes)
	binary.Write(buf, binary.LittleEndian, uint32(40))    // header size
	binary.Write(buf, binary.LittleEndian, int32(w))      // width
	binary.Write(buf, binary.LittleEndian, int32(h*2))    // height (doubled for ICO: XOR + AND mask)
	binary.Write(buf, binary.LittleEndian, uint16(1))     // planes
	binary.Write(buf, binary.LittleEndian, uint16(32))    // bpp
	binary.Write(buf, binary.LittleEndian, uint32(0))     // compression (BI_RGB)
	binary.Write(buf, binary.LittleEndian, uint32(w*h*4)) // image size
	binary.Write(buf, binary.LittleEndian, int32(0))      // x ppm
	binary.Write(buf, binary.LittleEndian, int32(0))      // y ppm
	binary.Write(buf, binary.LittleEndian, uint32(0))     // colors used
	binary.Write(buf, binary.LittleEndian, uint32(0))     // important colors

	// Pixel data (bottom-up, BGRA)
	for y := h - 1; y >= 0; y-- {
		for x := 0; x < w; x++ {
			r, g, b, a := img.At(bounds.Min.X+x, bounds.Min.Y+y).RGBA()
			buf.WriteByte(byte(b >> 8))
			buf.WriteByte(byte(g >> 8))
			buf.WriteByte(byte(r >> 8))
			buf.WriteByte(byte(a >> 8))
		}
	}

	// AND mask (all zeros = fully opaque, since we use 32-bit with alpha)
	andRowSize := ((w + 31) / 32) * 4
	andMask := make([]byte, andRowSize*h)
	buf.Write(andMask)

	return nil
}
