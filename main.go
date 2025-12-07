package main

import (
	"bufio"
	"context"
	"encoding/binary"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"github.com/gordonklaus/portaudio"
	"github.com/tommylay1902/vcalendar/wavwriter"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("missing required argument: output file name")
		return
	}

	fmt.Println("Recording. Type 'exit' and press Enter to stop.")

	fileName := os.Args[1]
	if !strings.HasSuffix(fileName, ".wav") {
		fileName += ".wav"
	}

	// Create WAV file (for local recording)
	f, err := os.Create(fileName)
	chk(err)
	defer f.Close()

	wavFmtChunk := wavwriter.Initialize(3, 44100, 32, 1) // 32-bit float, 44.1kHz
	wavwriter.WriteRiffHeader(f)
	wavwriter.WriteFmtChunk(f, &wavFmtChunk)
	wavwriter.WriteDataChunk(f, &wavFmtChunk)

	nSamples := 0
	defer wavwriter.FinalizeWritingToFile(f, &wavFmtChunk, nSamples)

	// Initialize PortAudio
	portaudio.Initialize()
	defer portaudio.Terminate()

	// Audio buffer
	in := make([]float32, 512) // Larger buffer for better performance
	stream, err := portaudio.OpenDefaultStream(1, 0, float64(wavFmtChunk.SampleRate), len(in), in)
	chk(err)
	defer stream.Close()

	chk(stream.Start())
	defer stream.Stop()

	// WebSocket connection to Vosk
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	c, _, err := websocket.Dial(ctx, "ws://localhost:2700", nil)
	chk(err)
	defer c.Close(websocket.StatusNormalClosure, "")

	// Send configuration to Vosk
	config := map[string]interface{}{
		"config": map[string]interface{}{
			"sample_rate": 16000.0, // Vosk expects 16kHz
		},
	}
	err = wsjson.Write(ctx, c, config)
	chk(err)

	// Setup stop channel
	stopChan := make(chan struct{})
	doneChan := make(chan struct{})

	// Start stdin reader goroutine
	go func() {
		reader := bufio.NewReader(os.Stdin)
		for {
			text, _ := reader.ReadString('\n')
			text = strings.TrimSpace(text)
			if strings.ToLower(text) == "exit" {
				fmt.Println("Stopping recording...")
				close(stopChan)
				return
			}
		}
	}()

	// Start WebSocket reader goroutine
	messageChan := make(chan interface{})
	errorChan := make(chan error)

	go func() {
		defer close(messageChan)
		defer close(errorChan)
		defer close(doneChan)

		for {
			var msg interface{}
			err := wsjson.Read(ctx, c, &msg)
			if err != nil {
				errorChan <- err
				return
			}
			select {
			case messageChan <- msg:
			case <-doneChan:
				return
			}
		}
	}()

	// Main recording loop
	fmt.Println("Recording started...")
	recording := true

	// For resampling: 44.1kHz -> 16kHz (downsample by factor ~2.75)
	resampleFactor := float64(wavFmtChunk.SampleRate) / 16000.0
	resampleAccumulator := 0.0

	// Buffer for 16kHz audio
	audio16k := make([]int16, 0, len(in))

	for recording {
		// Read audio from microphone
		err = stream.Read()
		if err != nil {
			log.Printf("Error reading audio: %v", err)
			break
		}

		// Write to WAV file (original 44.1kHz, 32-bit float)
		err = binary.Write(f, binary.LittleEndian, in)
		chk(err)
		nSamples += len(in)

		// Convert and resample for Vosk (44.1kHz float32 -> 16kHz int16)
		for _, sample := range in {
			resampleAccumulator += 1.0
			if resampleAccumulator >= resampleFactor {
				resampleAccumulator -= resampleFactor

				// Convert float32 (-1.0 to 1.0) to int16
				var intSample int16
				if sample > 1.0 {
					sample = 1.0
				} else if sample < -1.0 {
					sample = -1.0
				}
				intSample = int16(sample * 32767.0)
				audio16k = append(audio16k, intSample)
			}
		}

		// Send audio to Vosk when we have enough samples
		if len(audio16k) >= 160 { // ~10ms of 16kHz audio
			// Convert int16 to bytes (little-endian)
			audioBytes := make([]byte, len(audio16k)*2)
			for i, sample := range audio16k {
				audioBytes[i*2] = byte(sample)
				audioBytes[i*2+1] = byte(sample >> 8)
			}

			// Send raw audio to Vosk
			err = c.Write(ctx, websocket.MessageBinary, audioBytes)
			if err != nil {
				log.Printf("Error sending audio: %v", err)
				break
			}

			// Reset buffer
			audio16k = audio16k[:0]
		}

		// Check for messages or stop signal
		select {
		case msg := <-messageChan:
			handleVoskMessage(msg)
		case err := <-errorChan:
			if err != nil {
				log.Printf("WebSocket error: %v", err)
			}
			recording = false
		case <-stopChan:
			recording = false
		default:
			// Continue recording
		}
	}

	// Send EOF to Vosk
	if err := wsjson.Write(ctx, c, map[string]interface{}{"eof": 1}); err != nil {
		log.Printf("Error sending EOF: %v", err)
	}

	// Wait for final messages
	fmt.Println("Waiting for final transcriptions...")
	timeout := time.After(3 * time.Second)
	for {
		select {
		case msg := <-messageChan:
			handleVoskMessage(msg)
		case <-timeout:
			fmt.Println("Timeout waiting for final messages")
			goto cleanup
		case <-doneChan:
			goto cleanup
		}
	}

cleanup:
	fmt.Println("Recording saved to", fileName)
}

func handleVoskMessage(msg interface{}) {
	// Try to parse as JSON object
	if m, ok := msg.(map[string]interface{}); ok {
		if text, ok := m["text"].(string); ok && text != "" {
			fmt.Printf("\nFinal: %s\n", text)
		} else if partial, ok := m["partial"].(string); ok && partial != "" {
			fmt.Printf("\rPartial: %s", partial)
		}
	} else if str, ok := msg.(string); ok {
		fmt.Printf("Message: %s\n", str)
	}
}

func chk(err error) {
	if err != nil {
		panic(err)
	}
}
