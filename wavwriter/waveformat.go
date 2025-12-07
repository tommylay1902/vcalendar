package wavwriter

type WaveFormat struct {
	chunkSize     int
	AudioFormat   int
	SampleRate    int
	BitsPerSample int
	Channels      int
	blockAlign    int
	byteRate      int
}

func Initialize(audioFormat, sampleRate, bitsPerSample, channels int) WaveFormat {
	wav := WaveFormat{
		chunkSize:     16,
		AudioFormat:   audioFormat,
		SampleRate:    sampleRate,
		BitsPerSample: bitsPerSample,
		Channels:      channels,
		blockAlign:    channels * bitsPerSample / 8,
	}
	wav.byteRate = wav.SampleRate * wav.blockAlign
	return wav
}

func (wav WaveFormat) ChunkSize() int {
	return wav.chunkSize
}

func (wav WaveFormat) BlockAlign() int {
	return wav.blockAlign
}

func (wav WaveFormat) ByteRate() int {
	return wav.byteRate
}
