package wavwriter

import (
	"encoding/binary"
	"os"
)

func WriteRiffHeader(f *os.File) {
	_, err := f.WriteString("RIFF")
	chk(err)
	chk(binary.Write(f, binary.LittleEndian, int32(0))) // Placeholder for file size
	_, err = f.WriteString("WAVE")
	chk(err)
}

func WriteFmtChunk(f *os.File, wavFmtChunk *WaveFormat) {
	_, err := f.WriteString("fmt ")
	chk(err)
	chk(binary.Write(f, binary.LittleEndian, int32(16)))
	chk(binary.Write(f, binary.LittleEndian, int16(3)))
	chk(binary.Write(f, binary.LittleEndian, int16(wavFmtChunk.Channels)))
	chk(binary.Write(f, binary.LittleEndian, int32(wavFmtChunk.SampleRate)))
	chk(binary.Write(f, binary.LittleEndian, int32(wavFmtChunk.ByteRate())))
	chk(binary.Write(f, binary.LittleEndian, int16(wavFmtChunk.BlockAlign())))
	chk(binary.Write(f, binary.LittleEndian, int16(wavFmtChunk.BitsPerSample)))
}

func WriteDataChunk(f *os.File, wavFmtChunk *WaveFormat) {
	_, err := f.WriteString("data")
	chk(err)
	chk(binary.Write(f, binary.LittleEndian, int32(0)))
}

func FinalizeWritingToFile(f *os.File, wavFmtChunk *WaveFormat, nSamples int) {
	// Fill in the sizes
	fileSize := 36 + nSamples*(wavFmtChunk.BitsPerSample/8)*wavFmtChunk.Channels
	dataSize := nSamples * (wavFmtChunk.BitsPerSample / 8) * wavFmtChunk.Channels

	// Update RIFF chunk size
	_, err := f.Seek(4, 0)
	chk(err)
	chk(binary.Write(f, binary.LittleEndian, int32(fileSize)))

	// Update data chunk size
	_, err = f.Seek(40, 0)
	chk(err)
	chk(binary.Write(f, binary.LittleEndian, int32(dataSize)))

	chk(f.Close())
}

func chk(err error) {
	if err != nil {
		panic(err)
	}
}
