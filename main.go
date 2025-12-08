package main

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"syscall"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"github.com/gordonklaus/portaudio"
	"github.com/tommylay1902/vcalendar/voskutil"
	"github.com/tommylay1902/vcalendar/wavwriter"
)

func main() {
	// seed.SeedGCOperations()
	devNull, _ := os.OpenFile("/dev/null", os.O_WRONLY, 0666)
	syscall.Dup2(int(devNull.Fd()), int(os.Stderr.Fd()))
	devNull.Close()

	fmt.Println("Recording. Type 'exit' and press Enter to stop.")

	wavFmtChunk := wavwriter.Initialize(3, 16000, 16, 1) // 16-bit , 16kHz
	nSamples := 0

	// Initialize PortAudio
	portaudio.Initialize()
	defer portaudio.Terminate()

	// Audio buffer
	in := make([]int16, 1024) // Larger buffer for better performance
	stream, err := portaudio.OpenDefaultStream(1, 0, float64(wavFmtChunk.SampleRate), len(in), in)
	chk(err)
	defer stream.Close()

	chk(stream.Start())
	defer stream.Stop()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	// WebSocket connection to Vosk
	c, _, err := websocket.Dial(ctx, "ws://localhost:2700", nil)
	chk(err)
	defer c.Close(websocket.StatusNormalClosure, "")

	// Send configuration to Vosk
	config := map[string]any{
		"config": map[string]any{
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
	messageChan := make(chan any)
	errorChan := make(chan error)

	go func() {
		defer close(messageChan)
		defer close(errorChan)
		defer close(doneChan)

		for {
			var msg any
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

	for recording {
		// Read audio from microphone
		err = stream.Read()
		if err != nil {
			log.Printf("Error reading audio: %v", err)
			break
		}

		// Write to WAV file (original 44.1kHz, 32-bit float)
		nSamples += len(in)

		// Send audio to Vosk when we have enough samples
		if len(in) >= 160 { // ~10ms of 16kHz audio
			audioBytes := make([]byte, len(in)*2)
			for i, sample := range in {
				audioBytes[i*2] = byte(sample)
				audioBytes[i*2+1] = byte(sample >> 8)
			}

			// Send raw audio to Vosk
			err = c.Write(ctx, websocket.MessageBinary, audioBytes)
			if err != nil {
				log.Printf("Error sending audio: %v", err)
				break
			}
		}

		// Check for messages or stop signal
		select {
		case msg := <-messageChan:
			voskutil.HandleVoskMessage(msg)
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
	if err := wsjson.Write(ctx, c, map[string]any{"eof": 1}); err != nil {
		log.Printf("Error sending EOF: %v", err)
	}

	// Wait for final messages
	fmt.Println("Waiting for final transcriptions...")
	timeout := time.After(2 * time.Second)
	for {
		select {
		case msg := <-messageChan:
			voskutil.HandleVoskMessage(msg)
		case <-timeout:
			fmt.Println("Timeout waiting for final messages")
			goto cleanup
		case <-doneChan:
			goto cleanup
		}
	}

cleanup:

	c.CloseNow()
	fmt.Println("finished")
}

func chk(err error) {
	if err != nil {
		panic(err)
	}
}
