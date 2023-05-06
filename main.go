package main

import (
	"flag"
	"fmt"
	"github.com/fogleman/gg"
	"github.com/mjibson/go-dsp/fft"
	"github.com/mjibson/go-dsp/wav"
	"hash/fnv"
	"image"
	"image/color"
	"io"
	"log"
	"math"
	"math/cmplx"
	"os"
	"os/exec"
	"path/filepath"
)

type Shape struct {
	x, y  float64
	size  float64
	color color.RGBA
}

func main() {

	inputWAVFile := flag.String("input-file", "input.wav", "The file to use for input to the music video maker")
	frameOutputDir := flag.String("frame-dir", "frames", "directory used for frame output")
	outputFile := flag.String("output-file", "output.mp4", "file to write output to")
	windowSize := flag.Int("window-size", 4096, "window size")
	keepFrames := flag.Bool("keep-frames", false, "whether or not to keep the frames output directory, default: false")
	scaleFactor := flag.Int("scale-factor", 2, "adjust this value to change the shape movement")

	flag.Parse()

	// Read and process the WAV file
	f, err := os.Open(*inputWAVFile)
	if err != nil {
		fmt.Println("Error opening WAV file:", err)
		return
	}
	colorLog(colorGreen, fmt.Sprintf("successfully read %s", *inputWAVFile))
	defer f.Close()

	decoder, err := wav.New(f)
	if err != nil {
		panic(err)
	}

	// Analyze the audio data
	samples, err := decoder.ReadFloats(decoder.Samples)
	if err != nil {
		panic(err)
	}

	sampleBuf := make([]float64, len(samples))
	for i, sample := range samples {
		sampleBuf[i] = float64(sample)
	}

	sampleRate := int(decoder.SampleRate)
	duration := float64(len(samples)) / float64(sampleRate)
  numBins := 128
	spectrogram, frameDuration := createSpectrogram(sampleBuf, sampleRate, *windowSize)
	normalizedSpectrogram := normalizeSpectrogram(spectrogram, sampleRate, numBins)

	// generate PNG images
	if err := os.MkdirAll(*frameOutputDir, 0755); err != nil {
		fmt.Println("Error creating output directory:", err)
		return
	}

	nWorkers := 8 // Number of concurrent workers
	nFrames := len(normalizedSpectrogram)
	frameRate := 2.0 / frameDuration
	jobs := make(chan int, nFrames)
	results := make(chan error, nFrames)
	hashValue := generateHash(*inputWAVFile)

	// Start worker goroutines
	for w := 0; w < nWorkers; w++ {
		go func() {
			for i := range jobs {
				frame := normalizedSpectrogram[i]
				img := renderFrame(frame, i, nFrames, hashValue, float64(*scaleFactor))
				outputFile := filepath.Join(*frameOutputDir, fmt.Sprintf("frame%05d.png", i))
				err := gg.SavePNG(outputFile, img)
				results <- err
			}
		}()
	}

	// Send jobs
	colorLog(colorGreen, "rendering frames")
	progressBarWidth := 50
	for i := 0; i < nFrames; i++ {
		jobs <- i
	}

	close(jobs)

	// Receive results
	for i := 0; i < nFrames; i++ {
		err := <-results
		if err != nil {
			fmt.Printf("Error saving frame %d: %v\n", i, err)
		}
		progress := float64(i+1) / float64(nFrames)
		bar := int(progress * float64(progressBarWidth))

		fmt.Printf("\r[")
		for j := 0; j < bar; j++ {
			fmt.Printf("=")
		}
		for j := bar; j < progressBarWidth; j++ {
			fmt.Printf(" ")
		}
		// Print the progress percentage, the current frame, and the total frame count
		fmt.Printf("] %3.0f%% (%d/%d)", progress*100, i+1, nFrames)
	}

	// new line once progress is finished
	fmt.Printf("\n")

	// Generate output file
	err = createMP4(*inputWAVFile, *frameOutputDir, *outputFile, frameRate, duration)
	if err != nil {
		fmt.Println("Error creating file:", err)
		return
	}

	// remove frames
	if !*keepFrames {
		err := os.RemoveAll(*frameOutputDir)
		if err != nil {
			log.Fatal(err)
		}
		colorLog(colorGreen, "cleaning up...removed frames directory")
	} else {
		colorLog(colorYellow, "leaving frames behind")
	}

	colorLog(colorRed, fmt.Sprintf("your music video is ready!!: %s", *outputFile))
}

func renderFrame(frame []float64, frameIndex, totalFrames int, hashValue uint64, scaleFactor float64) image.Image {
	width := 1280
	if width%2 != 0 {
		width += 1 // Ensure width is divisible by 2
	}
	height := 720
	if height%2 != 0 {
		height += 1 // Ensure height is divisible by 2
	}

	dc := gg.NewContext(width, height)
	dc.SetRGB(0, 0, 0)
	dc.Clear()

	numBars := 62
	barWidth := float64(width) / float64(numBars)
	frameLen := len(frame)
	binSize := frameLen / numBars

	for i := 0; i < numBars; i++ {
		startBin := i * binSize
		endBin := (i + 1) * binSize

		// Calculate the average value of the frequency bins in this bar
		avgValue := 0.0
		for _, value := range frame[startBin:endBin] {
			avgValue += value
		}
		avgValue /= float64(binSize)

		// Apply the scaling factor to the average value
		avgValue *= scaleFactor

		// Cap the value at 1.0
		if avgValue > 1.0 {
			avgValue = 1.0
		}

		// Set color to a constant value
		colorValue := 1.0
		dc.SetRGB(colorValue, colorValue, colorValue)

		// Draw a rectangle based on the scaled average value
		x := float64(i) * barWidth
		y := float64(height) * (1.0 - avgValue)
		rectHeight := float64(height) - y
		dc.DrawRectangle(x, y, barWidth, rectHeight)
		dc.Fill()
	}

	return dc.Image()
}

func createSpectrogram(samples []float64, sampleRate int, windowSize int) ([][]float64, float64) {
	colorLog(colorGreen, "creating spectrogram")
	stepSize := windowSize // Set stepSize equal to windowSize, no overlap
	nFrames := (len(samples)-windowSize)/stepSize + 1

	spectrogram := make([][]float64, nFrames)
	for i := 0; i < nFrames; i++ {
		start := i * stepSize
		end := start + windowSize

		windowedSamples := applyHannWindow(samples[start:end])
		fftBuf := fft.FFTReal(windowedSamples)
		halfFFT := len(fftBuf) / 2
		magnitudes := make([]float64, halfFFT)

		for j, value := range fftBuf[:halfFFT] {
			magnitudes[j] = cmplx.Abs(value) / float64(windowSize)
		}

		spectrogram[i] = magnitudes
	}

	frameDuration := float64(stepSize) / float64(sampleRate)
	return spectrogram, frameDuration
}

func createMP4(inputWAVFile, frameOutputDir, outputMP4File string, frameRate float64, duration float64) error {
	colorLog(colorGreen, "sending frames to ffmpeg")
	ffmpegCmd := "ffmpeg"
	inputPattern := filepath.Join(frameOutputDir, "frame%05d.png")
	args := []string{
		"-r", fmt.Sprintf("%.2f", frameRate), // Frame rate
		"-thread_queue_size", "2048",
		"-i", inputPattern,
		"-i", inputWAVFile,
		"-c:v", "libx264", // Video codec
		"-c:a", "aac", // Audio codec
		"-pix_fmt", "yuv420p",
		"-y", outputMP4File,
	}

	cmd := exec.Command(ffmpegCmd, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("ffmpeg error: %v", err)
	}
	return nil
}

func applyHannWindow(samples []float64) []float64 {
	windowed := make([]float64, len(samples))
	for i := range samples {
		hann := 0.5 * (1 - math.Cos(2*math.Pi*float64(i)/float64(len(samples)-1)))
		windowed[i] = samples[i] * hann
	}

	return windowed
}

func hertzToMel(freq float64) float64 {
	return 1127.0 * math.Log10(1+freq/700.0)
}

func melToHertz(mel float64) float64 {
	return 700.0 * (math.Pow(10, mel/1127.0) - 1)
}

func redistributeMel(spectrogram [][]float64, sampleRate int, numBins int) [][]float64 {
	melSpectrogram := make([][]float64, len(spectrogram))

	lowFreqMel := hertzToMel(0)
	highFreqMel := hertzToMel(float64(sampleRate / 2))
	melStep := (highFreqMel - lowFreqMel) / float64(numBins+1)

	for i, frame := range spectrogram {
		melBins := make([]float64, numBins)
		for j := 0; j < numBins; j++ {
			melLow := melToHertz(lowFreqMel + melStep*float64(j))
			melHigh := melToHertz(lowFreqMel + melStep*float64(j+1))
			startBin := int(melLow / float64(sampleRate/2) * float64(len(frame)))
			endBin := int(melHigh / float64(sampleRate/2) * float64(len(frame)))

			avgValue := 0.0
			for _, value := range frame[startBin:endBin] {
				avgValue += value
			}
			avgValue /= float64(endBin - startBin)
			melBins[j] = avgValue
		}
		melSpectrogram[i] = melBins
	}

	return melSpectrogram
}

func normalizeSpectrogram(spectrogram [][]float64, sampleRate int, numBins int) [][]float64 {
	// Redistribute the frequencies using the Mel scale
	melSpectrogram := redistributeMel(spectrogram, sampleRate, numBins)

	// Normalize the mel spectrogram as before
	normalizedSpectrogram := make([][]float64, len(melSpectrogram))
	globalMaxMagnitude := 0.0
	globalMinMagnitude := math.MaxFloat64

	for _, frame := range melSpectrogram {
		for _, magnitude := range frame {
			if magnitude > globalMaxMagnitude {
				globalMaxMagnitude = magnitude
			}
			if magnitude < globalMinMagnitude {
				globalMinMagnitude = magnitude
			}
		}
	}

	for i, frame := range melSpectrogram {
		normalizedFrame := make([]float64, len(frame))
		for j, magnitude := range frame {
			normalizedMagnitude := (magnitude - globalMinMagnitude) / (globalMaxMagnitude - globalMinMagnitude)
			normalizedMagnitude = math.Log10(normalizedMagnitude*15.0 + 1.0)
			normalizedFrame[j] = normalizedMagnitude
		}
		normalizedSpectrogram[i] = normalizedFrame
	}

	colorLog(colorGreen, "normalized spectrogram")
	return normalizedSpectrogram
}

func generateHash(inputWAVFile string) uint64 {
	f, err := os.Open(inputWAVFile)
	if err != nil {
		colorLog(colorRed, fmt.Sprintf("Error opening WAV file:", err))
		return 0
	}
	defer f.Close()

	h := fnv.New64a()
	if _, err := io.Copy(h, f); err != nil {
		colorLog(colorRed, fmt.Sprintf("Error generating hash:", err))
		return 0
	}
	colorLog(colorGreen, "generated hash")
	return h.Sum64()
}
