package main

import (
	"fmt"
	"image/color"
	"image/png"
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/vpbukhti/huaskii/renderer"
	"golang.org/x/image/font/sfnt"
)

func main() {
	// Parse arguments: main_text filler_text fill_scale [num_rows]
	if len(os.Args) < 4 {
		fmt.Println("Usage: huaskii <main_text> <filler_text> <fill_scale> [num_rows]")
		fmt.Println()
		fmt.Println("  main_text   - Text to render as large outlines")
		fmt.Println("  filler_text - Text to repeat along the curves")
		fmt.Println("  fill_scale  - Size of filler relative to stroke (0.05 to 1.0)")
		fmt.Println("  num_rows    - Number of rows to fill (optional, default: auto)")
		fmt.Println()
		fmt.Println("Example: huaskii Hello world 0.5")
		fmt.Println("Example: huaskii Hello world 0.5 3")
		os.Exit(1)
	}

	mainText := os.Args[1]
	fillerText := strings.Trim(os.Args[2], " ") + "   "
	fillScale, err := strconv.ParseFloat(os.Args[3], 64)
	if err != nil {
		log.Fatalf("invalid fill_scale: %v", err)
	}

	if fillScale <= 0 || fillScale > 1 {
		log.Fatalf("fill_scale must be between 0.01 and 1.0")
	}

	numRows := 0 // 0 = auto
	if len(os.Args) >= 5 {
		numRows, err = strconv.Atoi(os.Args[4])
		if err != nil {
			log.Fatalf("invalid num_rows: %v", err)
		}
	}

	// Load the font file
	fontData, err := os.ReadFile("assets/Roboto-VariableFont_wdth,wght.ttf")
	if err != nil {
		log.Fatalf("failed to read font: %v", err)
	}

	font, err := sfnt.Parse(fontData)
	if err != nil {
		log.Fatalf("failed to parse font: %v", err)
	}

	// Settings
	fontSize := 1000.0
	strokeWidth := 50.0
	padding := 100.0

	// Calculate canvas dimensions based on text
	textWidth := renderer.MeasureText(font, mainText, fontSize)
	width := int(textWidth + padding*2)
	height := int(fontSize + padding*2) // 1.4 factor for ascenders/descenders

	// Create canvas
	canvas := renderer.NewCanvas(width, height)
	canvas.Fill(color.White)

	// Create renderer
	textRenderer := renderer.NewTextRenderer(font, canvas)

	// Settings
	settings := renderer.RenderSettings{
		MainText:    mainText,
		FillerText:  fillerText,
		FontSize:    fontSize,
		StrokeWidth: strokeWidth,
		FillScale:   fillScale,
		NumRows:     numRows,
	}

	// Center vertically
	baseline := float64(height)/2 + fontSize*0.3

	// Render
	textRenderer.RenderTextWithFiller(settings, padding, baseline, color.RGBA{0, 0, 0, 255})

	// Ensure output directory exists
	os.MkdirAll("output", 0755)

	// Save to PNG
	outFile, err := os.Create("output/output.png")
	if err != nil {
		log.Fatalf("failed to create output file: %v", err)
	}
	defer outFile.Close()

	if err := png.Encode(outFile, canvas.Img); err != nil {
		log.Fatalf("failed to encode PNG: %v", err)
	}

	log.Printf("Rendered '%s' with filler '%s' (scale %.2f) to output/output.png", mainText, fillerText, fillScale)
	log.Println("Profiles written to output/cpu.prof and output/mem.prof")
	log.Println("View with: go tool pprof -http=:8080 output/cpu.prof")
}
