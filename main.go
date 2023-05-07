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
	"runtime"
	"strconv"
	"strings"
)

type Shape struct {
	x, y  float64
	size  float64
	color color.RGBA
}

func main() {

	// file based flags
	inputWAVFile := flag.String("input-file", "input.wav", "The file to use for input to the music video maker")
	frameOutputDir := flag.String("frame-dir", "frames", "directory used for frame output")
	outputFile := flag.String("output-file", "output.mp4", "file to write output to")
	keepFrames := flag.Bool("keep-frames", false, "whether or not to keep the frames output directory")

	// windowSize is how big each "window" in the sample is.
	windowSize := flag.Int("window-size", 4096, "sample window size")
	smoothingSize := flag.Float64("smoothing-size", 0.5, "smoothing size, attempts to smooth out movement between frames")

	// scaleFactor increases the responsiveness of each bar in the EQ. Set it too high and most bars will just stay at maximum height
	scaleFactor := flag.Float64("scale-factor", 0.0, "adjust this value to change the shape movement, you may want to up the hpf when upping scale-factor")
	hpf := flag.Int("hpf", 0, "high pass filter, used to cut off lower frequencies which can overpower the bottom end of the visualization")

	// visualization options
	barCount := flag.Int("bar-count", 64, "number of eq bars to render, more bars equals thinner bars overall. 32, 64, 128 are good places to start")
	rainbow := flag.Bool("rainbow", false, "rainbow mode does exactly what it says, applies a gradient rainbow to the EQ")
	baseColor := flag.String("base-color", "ffffff", "base color used to color the EQ bars, accepts a hex value")

	flag.Parse()

	// Read and process the WAV file
	f, err := os.Open(*inputWAVFile)
	if err != nil {
		fmt.Println("Error opening WAV file:", err)
		return
	}
	colorLog(colorCyan, fmt.Sprintf("successfully read %s", *inputWAVFile))
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
	colorLog(colorCyan, fmt.Sprintf("decoded sample %s", *inputWAVFile))

	sampleRate := int(decoder.SampleRate)
	duration := float64(len(samples)) / float64(sampleRate)
	numBins := 128
	spectrogram := make([][]float64, len(samples))
	normalizedSpectrogram := make([][]float64, len(samples))
	frameDuration := 0.0

	if *hpf == 0 {
		colorLog(colorCyan, fmt.Sprintf("bypassing high-pass filter"))
		spectrogram, frameDuration = createSpectrogram(sampleBuf, sampleRate, *windowSize)
		normalizedSpectrogram = normalizeSpectrogram(spectrogram, sampleRate, numBins)
	} else {
		// Apply the high-pass filter
		colorLog(colorCyan, fmt.Sprintf("applying high-pass filter"))
		filteredSamples := highPassFilter(sampleBuf, sampleRate, float64(*hpf))
		spectrogram, frameDuration = createSpectrogram(filteredSamples, sampleRate, *windowSize)
		normalizedSpectrogram = normalizeSpectrogram(spectrogram, sampleRate, numBins)
	}

	// Apply interpolation to the normalized spectrogram
	interpolationFactor := *smoothingSize // Adjust this value to control the smoothness (0 to 1)
	smoothedSpectrogram := make([][]float64, 0, len(normalizedSpectrogram)*2-1)

	for i := 0; i < len(normalizedSpectrogram)-1; i++ {
		smoothedSpectrogram = append(smoothedSpectrogram, normalizedSpectrogram[i])
		interpolatedFrame := interpolateFrames(normalizedSpectrogram[i], normalizedSpectrogram[i+1], interpolationFactor)
		smoothedSpectrogram = append(smoothedSpectrogram, interpolatedFrame)
	}
	smoothedSpectrogram = append(smoothedSpectrogram, normalizedSpectrogram[len(normalizedSpectrogram)-1])
	nFrames := len(smoothedSpectrogram)

	// generate PNG images
	if err := os.MkdirAll(*frameOutputDir, 0755); err != nil {
		fmt.Println("Error creating output directory:", err)
		return
	}
	colorLog(colorCyan, fmt.Sprintf("created output dir for frames %s", *frameOutputDir))

	nWorkers := runtime.NumCPU() // Number of concurrent workers
	frameRate := 4.0 / frameDuration
	jobs := make(chan int, nFrames)
	results := make(chan error, nFrames)
	hashValue := generateHash(*inputWAVFile)

	// Start worker goroutines
	for w := 0; w < nWorkers; w++ {
		go func() {
			for i := range jobs {
				frame := smoothedSpectrogram[i] // Use smoothedSpectrogram
				img := renderFrame(frame, i, nFrames, hashValue, *scaleFactor, *barCount, *rainbow, *baseColor)
				outputFile := filepath.Join(*frameOutputDir, fmt.Sprintf("frame%05d.png", i))
				err := gg.SavePNG(outputFile, img)
				results <- err
			}

		}()
	}

	// Send jobs
	colorLog(colorCyan, "rendering frames")
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
		colorLog(colorCyan, "cleaning up...removing frames directory")
		err := os.RemoveAll(*frameOutputDir)
		if err != nil {
			log.Fatal(err)
		}
		colorLog(colorCyan, "frames deleted")
	} else {
		colorLog(colorYellow, "leaving frames behind")
	}

	colorLog(colorCyan, fmt.Sprintf("Your music video is ready at %s", *outputFile))
}

func interpolateFrames(frame1, frame2 []float64, factor float64) []float64 {
	interpolatedFrame := make([]float64, len(frame1))
	for i := range frame1 {
		interpolatedFrame[i] = frame1[i]*(1-factor) + frame2[i]*factor
	}
	return interpolatedFrame
}

func renderFrame(frame []float64, frameIndex, totalFrames int, hashValue uint64, scaleFactor float64, barCount int, rainbow bool, baseColor string) image.Image {
	width := 1920
	if width%2 != 0 {
		width += 1 // Ensure width is divisible by 2
	}
	height := 1080
	if height%2 != 0 {
		height += 1 // Ensure height is divisible by 2
	}

	dc := gg.NewContext(width, height)
	dc.SetRGB(0, 0, 0)
	dc.Clear()

	barWidth := float64(width) / float64(barCount)
	frameLen := len(frame)
	binSize := frameLen / barCount

	for i := 0; i < barCount; i++ {
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

		r, g, b, err := parseHexColor(baseColor)
		if err != nil {
			fmt.Printf("error parsing hex baseColor value: %s\n", err)
		}

		// SetRBB expects values between 0 and 1, so we have to divide by 255
		dc.SetRGB(float64(r)/255, float64(g)/255, float64(b)/255)

		if rainbow {
			// Create a gradient for the EQ bars from red to violet
			red := math.Sin(0.1*float64(i)+0)*127 + 128
			green := math.Sin(0.1*float64(i)+2*math.Pi/3)*127 + 128
			blue := math.Sin(0.1*float64(i)+4*math.Pi/3)*127 + 128
			dc.SetRGB(red/255, green/255, blue/255)
		}

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
	colorLog(colorCyan, "creating spectrogram")
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
	colorLog(colorCyan, "sending frames to ffmpeg")
	ffmpegCmd := "ffmpeg"
	inputPattern := filepath.Join(frameOutputDir, "frame%05d.png")
	args := []string{
		"-r", fmt.Sprintf("%.2f", frameRate), // Frame rate
		"-thread_queue_size", "8192",
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

	colorLog(colorCyan, "normalized spectrogram")
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
	colorLog(colorCyan, "generated hash")
	return h.Sum64()
}

func highPassFilter(samples []float64, sampleRate int, cutoffFrequency float64) []float64 {
	alpha := math.Exp(-2 * math.Pi * cutoffFrequency / float64(sampleRate))
	filteredSamples := make([]float64, len(samples))
	for i := 1; i < len(samples); i++ {
		filteredSamples[i] = alpha * (filteredSamples[i-1] + samples[i] - samples[i-1])
	}
	return filteredSamples
}

func parseHexColor(s string) (uint32, uint32, uint32, error) {
	if strings.HasPrefix(s, "#") {
		s = s[1:]
	}

	if len(s) != 6 {
		return 0, 0, 0, fmt.Errorf("invalid format")
	}

	redHex := s[0:2]
	greenHex := s[2:4]
	blueHex := s[4:6]

	red, err := strconv.ParseUint(redHex, 16, 8)
	if err != nil {
		return 0, 0, 0, err
	}

	green, err := strconv.ParseUint(greenHex, 16, 8)
	if err != nil {
		return 0, 0, 0, err
	}

	blue, err := strconv.ParseUint(blueHex, 16, 8)
	if err != nil {
		return 0, 0, 0, err
	}

	return uint32(red), uint32(green), uint32(blue), nil
}
