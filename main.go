package main

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"os"
	"strings"
	"syscall"

	"github.com/gordonklaus/portaudio"
	"github.com/tommylay1902/vcalendar/wavwriter"
)

func main() {

	devNull, _ := os.OpenFile("/dev/null", os.O_WRONLY, 0666)
	syscall.Dup(int(os.Stderr.Fd()))
	syscall.Dup2(int(devNull.Fd()), int(os.Stderr.Fd()))

	if len(os.Args) < 2 {
		fmt.Println("missing required argument: output file name")
		return
	}

	fmt.Println("Recording. Type 'exit' and press Enter to stop.")

	fileName := os.Args[1]
	if !strings.HasSuffix(fileName, ".wav") {
		fileName += ".wav"
	}

	f, err := os.Create(fileName)
	chk(err)

	wavwriter.WriteRiffHeader(f)

	wavFmtChunk := wavwriter.Initialize(3, 44100, 32, 1)
	wavwriter.WriteFmtChunk(f, &wavFmtChunk)

	wavwriter.WriteDataChunk(f, &wavFmtChunk)

	nSamples := 0
	defer wavwriter.FinalizeWritingToFile(f, &wavFmtChunk, nSamples)

	portaudio.Initialize()
	defer portaudio.Terminate()
	in := make([]float32, 64)
	stream, err := portaudio.OpenDefaultStream(1, 0, float64(wavFmtChunk.SampleRate), len(in), in)
	chk(err)
	defer stream.Close()

	chk(stream.Start())

	stopChan := make(chan bool, 1)

	go func() {
		reader := bufio.NewReader(os.Stdin)
		for {
			text, _ := reader.ReadString('\n')
			text = strings.TrimSpace(text)
			if strings.ToLower(text) == "exit" {
				fmt.Println("Stopping recording...")
				stopChan <- true
				return
			}
		}
	}()

	recording := true
	for recording {
		chk(stream.Read())
		chk(binary.Write(f, binary.LittleEndian, in))
		nSamples += len(in)
		select {
		case <-stopChan:
			recording = false
		default:
			// Continue recording
		}
	}

	chk(stream.Stop())
	fmt.Println("Recording saved to", fileName)
}

func chk(err error) {
	if err != nil {
		panic(err)
	}
}
