package main

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"github.com/gordonklaus/portaudio"
	"github.com/kelindar/search"
	"github.com/qdrant/go-client/qdrant"
	"github.com/tommylay1902/vcalendar/voskutil"
)

func ReadUserInput(stopChan chan struct{}) {
	defer close(stopChan)
	reader := bufio.NewReader(os.Stdin)
	for {
		text, _ := reader.ReadString('\n')
		text = strings.TrimSpace(text)
		if strings.ToLower(text) == "exit" {
			fmt.Println("Stopping recording...")
			return
		}
	}
}

func WriteWebsocket(messageChan chan any, errorChan chan error, doneChan chan struct{}, ctx context.Context, c *websocket.Conn) {
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
}

func RecordAudio(gc GcClient, embedder *search.Vectorizer, qc *qdrant.Client, recording bool, stream *portaudio.Stream, in []int16,
	ctx context.Context, c *websocket.Conn,
	messageChan chan any, errorChan chan error, stopChan chan struct{}) {

	for recording {
		// Read audio from microphone
		err := stream.Read()
		if err != nil {
			log.Printf("Error reading audio: %v", err)
			break
		}

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
			// voskutil.HandleVoskMessage(msg)
			finalText := voskutil.HandleVoskMessage(msg)
			operation := GetOperation(qc, finalText, embedder)
			if operation == "List" {
				gc.GetEventsForTheDay()
			} else if operation == "Add" {
				fmt.Println("Creating event...")
			} else if operation == "Delete" {
				fmt.Println("Deleting event...")
			}
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
}
