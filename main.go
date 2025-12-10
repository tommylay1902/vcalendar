package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"github.com/gordonklaus/portaudio"
	"github.com/tommylay1902/vcalendar/seed"
	"github.com/tommylay1902/vcalendar/wavwriter"
)

func main() {
	gc := Initialize()
	seed.SeedGCOperations()
	IgnoreAudioWarnings()
	qc := SetupQdrantClient()
	embedder := SetupEmbeddingClient()

	fmt.Println("Recording. Type 'exit' and press Enter to stop.")
	wavFmtChunk := wavwriter.Initialize(3, 16000, 16, 1) // 16-bit , 16kHz

	// Initialize PortAudio
	portaudio.Initialize()
	defer portaudio.Terminate()

	// Audio buffer
	in := make([]int16, 2048) // Larger buffer for better performance
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

	writeCtx, cancel := context.WithTimeout(ctx, 100*time.Millisecond)
	err = wsjson.Write(writeCtx, c, config)
	chk(err)

	// Setup stop channel
	stopChan := make(chan struct{})
	doneChan := make(chan struct{})

	go ReadUserInput(stopChan)

	messageChan := make(chan any)
	errorChan := make(chan error)

	go WriteWebsocket(messageChan, errorChan, doneChan, ctx, c)

	fmt.Println("Recording started...")
	recording := true
	// need to break up the logic in this, too many moving parts
	RecordAudio(gc, embedder, qc, recording, stream, in, ctx,
		c, messageChan, errorChan, stopChan)

	// Send EOF to Vosk
	if err := wsjson.Write(ctx, c, map[string]any{"eof": 1}); err != nil {
		log.Printf("Error sending EOF: %v", err)
	}

	// cleanup:
	//
	//	c.CloseNow()
	//	fmt.Println("finished")
}

func chk(err error) {
	if err != nil {
		panic(err)
	}
}
