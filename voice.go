package nlp

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"
)

type VoiceSession struct {
	sync.Mutex
	pipeWriter io.Writer
	pipeReader io.Reader
	PipeOut    chan []byte // Ignore this for now
	listening  bool
	started    time.Time
	process    *os.Process
	err        error
	buf        bytes.Buffer
	encoder    *EncodeSession // WAV encoder session
	framesSent int
}

func Voice(encoder *EncodeSession) *VoiceSession {
	session := &VoiceSession{
		encoder: encoder,
	}

	go session.start()
	log.Println("Voice Session Started.")
	return session
}

func (v *VoiceSession) start() {
	// Reset listening state
	defer func() {
		v.Lock()
		v.listening = false
		v.Unlock()
	}()

	v.Lock()
	v.listening = true

	python := exec.Command("python", "C:\\Users\\NotMyPc\\Documents\\Projects\\python\\discordBot-NLP\\voice_recognition.py")

	stdin, err := python.StdinPipe()
	if err != nil {
		v.Unlock()
		log.Println("StdinPipe Error:", err)
		return
	}

	stdout, err := python.StdoutPipe()
	if err != nil {
		v.Unlock()
		log.Println("StdoutPipe Error:", err)
		return
	}
	log.Println(stdout)

	stderr, err := python.StderrPipe()
	if err != nil {
		v.Unlock()
		log.Println("StderrPipe Error:", err)
		return
	}

	err = python.Start()
	if err != nil {
		v.Unlock()
		log.Println("RunStart Error:", err)
		return
	}

	v.pipeWriter = stdin
	v.started = time.Now()
	v.process = python.Process
	v.Unlock()

	var wg sync.WaitGroup
	wg.Add(1)
	go v.readStderr(stderr, &wg)

	wg.Add(1)
	go v.readStdout(stdout, &wg)

	// Continuous audio streaming loop
	for {
		v.Lock()
		if !v.listening {
			v.Unlock()
			break
		}
		v.SendAudio()
		log.Println("Audio sent to Python script.")
		v.Unlock()
		// Add a small sleep to prevent high CPU usage
		time.Sleep(100 * time.Millisecond)
	}

	wg.Wait()
	err = python.Wait()
	if err != nil {
		if err.Error() != "signal: killed" {
			v.Lock()
			v.err = err
			v.Unlock()
		}
	}
}

// Stop stops the encoding session
func (v *VoiceSession) Stop() error {
	v.Lock()
	defer v.Unlock()
	return v.process.Kill()
}

func (v *VoiceSession) SendAudio() {
	wav, err := v.encoder.WAVFrame()
	if err != nil {
		log.Println(err)
	}
	// Write the frame data to the pipeWriter
	v.pipeWriter.Write(wav)
	log.Println("Audio sent to Python script.")

	// Increment the frames sent counter
	v.Lock()
	v.framesSent++
	v.Unlock()
}

// readStderr processes stderr output from the Python script
func (v *VoiceSession) readStderr(stderr io.ReadCloser, wg *sync.WaitGroup) {
	defer wg.Done()
	scanner := bufio.NewScanner(stderr)
	for scanner.Scan() {
		log.Printf("Python stderr: %s\n", scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		log.Printf("Error reading stderr: %v\n", err)
	}
}

// readStdout processes stdout output from the Python script
func (v *VoiceSession) readStdout(stdout io.ReadCloser, wg *sync.WaitGroup) {
	defer wg.Done()
	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		if strings.Contains(scanner.Text(), "Recognizer text:") {
			recognizedText := strings.Split(scanner.Text(), ":")[1]
			recognizedText = strings.TrimSpace(recognizedText)
			fmt.Printf("Recognized Text: %s\n", recognizedText)
		} else {
			fmt.Printf("Python stdout: %s\n", scanner.Text())
		}
	}
	if err := scanner.Err(); err != nil {
		log.Printf("Error reading stdout: %v\n", err)
	}
}

// Listening returns whether the session is currently listening
func (v *VoiceSession) Listening() (listening bool) {
	v.Lock()
	listening = v.listening
	v.Unlock()
	return
}
