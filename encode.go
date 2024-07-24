package nlp

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"log"
	"os"
	"os/exec"
	"sync"
	"time"
)

// AudioApplication is an application profile for audio encoding
type AudioApplication string

// ErrBadFrame is returned when an audio frame is invalid
var ErrBadFrame = errors.New("Bad Frame")

type Frame struct {
	data     []byte
	metaData bool
}

type EncodeSession struct {
	sync.Mutex
	PipeIn       chan []byte
	PipeOut      chan []byte
	running      bool
	started      time.Time
	frameChannel chan *Frame
	process      *os.Process
	lastFrame    int
	err          error
	// buffer that stores unread bytes (not full frames)
	// used to implement io.Reader
	buf bytes.Buffer
}

func Encode() *EncodeSession {
	pipeIn := make(chan []byte)
	pipeOut := make(chan []byte)

	session := &EncodeSession{
		frameChannel: make(chan *Frame, 100),
		PipeIn:       pipeIn,
		PipeOut:      pipeOut,
	}

	log.Println("Encode Session Started.")

	go session.run()

	return session
}

func (e *EncodeSession) run() {
	// Reset running state
	defer func() {
		e.Lock()
		e.running = false
		e.Unlock()
	}()

	e.Lock()
	e.running = true

	args := []string{
		"-i", "pipe:0", // Input from pipe
		"-f", "wav", // Output format: WAV
		"-ar", "16000", // Sample rate
		"-ac", "1", // Number of audio channels (mono)
		"pipe:1", // Output to pipe
	}

	ffmpeg := exec.Command("ffmpeg", args...)

	stdinPipe, err := ffmpeg.StdinPipe()
	if err != nil {
		e.Unlock()
		log.Println("StdinPipe Error:", err)
		close(e.frameChannel)
		return
	}

	stdout, err := ffmpeg.StdoutPipe()
	if err != nil {
		e.Unlock()
		log.Println("StdoutPipe Error:", err)
		close(e.frameChannel)
		return
	}

	stderr, err := ffmpeg.StderrPipe()
	if err != nil {
		e.Unlock()
		log.Println("StderrPipe Error:", err)
		close(e.frameChannel)
		return
	}

	// Starts the ffmpeg command
	err = ffmpeg.Start()
	if err != nil {
		e.Unlock()
		log.Println("RunStart Error:", err)
		close(e.frameChannel)
		return
	}

	e.started = time.Now()

	e.process = ffmpeg.Process
	e.Unlock()

	var wg sync.WaitGroup
	wg.Add(1)
	go e.readStderr(stderr, &wg)

	defer close(e.frameChannel)

	go func() {
		for data := range e.PipeIn {
			_, err := stdinPipe.Write(data)
			if err != nil {
				log.Printf("Error writing to stdin: %v", err)
				break
			}
			log.Println("Read stdin")
		}
		stdinPipe.Close()
	}()

	e.readStdout(stdout)
	wg.Wait()
	err = ffmpeg.Wait()
	if err != nil {
		log.Println(err)
		if err.Error() != "signal: killed" {
			e.Lock()
			e.err = err
			e.Unlock()
		}
	}
}

func (e *EncodeSession) readStdout(stdout io.ReadCloser) {
	buf := bufio.NewReader(stdout)
	for {
		data, err := buf.ReadBytes('\n')
		if err != nil {
			if err == io.EOF {
				break
			}
			log.Printf("Error reading from stdout: %v", err)
			break
		}
		e.PipeOut <- data
	}
}

func (e *EncodeSession) readStderr(stderr io.ReadCloser, wg *sync.WaitGroup) {
	defer wg.Done()
	buf := bufio.NewReader(stderr)
	for {
		line, err := buf.ReadBytes('\n')
		if err != nil {
			if err == io.EOF {
				break
			}
			log.Printf("Error reading from stderr: %v", err)
			break
		}
		log.Printf("ffmpeg stderr: %s", line)
	}
}

// Stop stops the encoding session
func (e *EncodeSession) Stop() error {
	e.Lock()
	defer e.Unlock()
	return e.process.Kill()
}

// ReadFrame blocks until a frame is read or there are no more frames
func (e *EncodeSession) ReadFrame() (frame []byte, err error) {
	f := <-e.frameChannel
	if f == nil {
		return nil, io.EOF
	}

	return f.data, nil
}

// Running returns true if running
func (e *EncodeSession) Running() (running bool) {
	e.Lock()
	running = e.running
	e.Unlock()
	return
}

// Cleanup cleans up the encoding session
func (e *EncodeSession) Cleanup() {
	e.Stop()

	for range e.frameChannel {
		// empty till closed
	}
}

func (e *EncodeSession) WAVFrame() (frame []byte, err error) {
	f := <-e.frameChannel
	if f == nil {
		return nil, io.EOF
	}
	if f.metaData {
		// Return the next one then...
		return e.WAVFrame()
	}
	if len(f.data) < 44 { // Minimum WAV header size is 44 bytes
		return nil, ErrBadFrame
	}
	return f.data[44:], nil // Skip WAV header (44 bytes)
}

func (e *EncodeSession) writeWAVFrame(pcmFrame []byte) error {
	// Write WAV header
	var headerBuf bytes.Buffer
	writeWAVHeader(&headerBuf, len(pcmFrame))

	// Write audio data
	var frameBuf bytes.Buffer
	_, err := frameBuf.Write(pcmFrame)
	if err != nil {
		log.Println("Error writing frameBuf.Write(pcmFrame)")
		return err
	}

	// Combine header and audio data
	_, err = headerBuf.Write(frameBuf.Bytes())
	if err != nil {
		log.Println("Error writing headerBuf.Write(frameBuf.Bytes())")
		return err
	}

	e.frameChannel <- &Frame{headerBuf.Bytes(), false}
	log.Println("Success writting Wav Frame.")

	e.Lock()
	e.lastFrame++
	e.Unlock()

	return nil
}

func writeWAVHeader(buf *bytes.Buffer, dataSize int) {
	const (
		riffID        = "RIFF"
		wavID         = "WAVE"
		fmtID         = "fmt "
		dataID        = "data"
		fmtSize       = 16
		audioFormat   = 1 // PCM
		numChannels   = 1
		sampleRate    = 16000
		bitsPerSample = 16
		blockAlign    = numChannels * bitsPerSample / 8
		byteRate      = sampleRate * blockAlign
	)

	// RIFF chunk descriptor
	buf.WriteString(riffID)
	binary.Write(buf, binary.LittleEndian, uint32(36+dataSize)) // Chunk size
	buf.WriteString(wavID)

	// Format sub-chunk
	buf.WriteString(fmtID)
	binary.Write(buf, binary.LittleEndian, uint32(fmtSize)) // Subchunk1 size
	binary.Write(buf, binary.LittleEndian, uint16(audioFormat))
	binary.Write(buf, binary.LittleEndian, uint16(numChannels))
	binary.Write(buf, binary.LittleEndian, uint32(sampleRate))
	binary.Write(buf, binary.LittleEndian, uint32(byteRate))
	binary.Write(buf, binary.LittleEndian, uint16(blockAlign))
	binary.Write(buf, binary.LittleEndian, uint16(bitsPerSample))

	// Data sub-chunk
	buf.WriteString(dataID)
	binary.Write(buf, binary.LittleEndian, uint32(dataSize)) // Subchunk2 size
}
