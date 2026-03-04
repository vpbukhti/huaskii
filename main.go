package main

import (
	"fmt"
	"image/color"
	"image/png"
	"log"
	"os"
	"runtime/pprof"
	"strconv"

	"github.com/vpbukhti/huaskii/renderer"
	"golang.org/x/image/font/sfnt"
)

func main() {
	// CPU profiling
	cpuFile, err := os.Create("output/cpu.prof")
	if err != nil {
		log.Fatal(err)
	}
	defer cpuFile.Close()
	pprof.StartCPUProfile(cpuFile)
	defer pprof.StopCPUProfile()

	// Parse arguments: main_text filler_text fill_scale
	if len(os.Args) < 4 {
		fmt.Println("Usage: huaskii <main_text> <filler_text> <fill_scale>")
		fmt.Println()
		fmt.Println("  main_text   - Text to render as large outlines")
		fmt.Println("  filler_text - Text to repeat along the curves")
		fmt.Println("  fill_scale  - Size of filler relative to stroke (0.05 to 1.0)")
		fmt.Println("                1.0 = single row, 0.5 = 2 rows, 0.25 = 4 rows")
		fmt.Println()
		fmt.Println("Example: huaskii Hello world 0.5")
		os.Exit(1)
	}

	mainText := os.Args[1]
	fillerText := os.Args[2]
	fillScale, err := strconv.ParseFloat(os.Args[3], 64)
	if err != nil {
		log.Fatalf("invalid fill_scale: %v", err)
	}

	if fillScale <= 0 || fillScale > 1 {
		log.Fatalf("fill_scale must be between 0.01 and 1.0")
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

	// Create canvas (high resolution)
	width, height := 4200, 1200
	canvas := renderer.NewCanvas(width, height)
	canvas.Fill(color.White)

	// Create renderer
	textRenderer := renderer.NewTextRenderer(font, canvas)

	// Settings
	settings := renderer.RenderSettings{
		MainText:    mainText,
		FillerText:  fillerText,
		FontSize:    540.0,
		StrokeWidth: 48.0,
		FillScale:   fillScale,
	}

	// Center vertically
	baseline := float64(height)/2 + settings.FontSize*0.3

	// Render
	textRenderer.RenderTextWithFiller(settings, 50, baseline, color.RGBA{0, 0, 0, 255})

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

	// Memory profiling
	memFile, err := os.Create("output/mem.prof")
	if err != nil {
		log.Fatal(err)
	}
	defer memFile.Close()
	pprof.WriteHeapProfile(memFile)

	log.Printf("Rendered '%s' with filler '%s' (scale %.2f) to output/output.png", mainText, fillerText, fillScale)
	log.Println("Profiles written to output/cpu.prof and output/mem.prof")
	log.Println("View with: go tool pprof -http=:8080 output/cpu.prof")
}
