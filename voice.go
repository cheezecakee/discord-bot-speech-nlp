package main

import (
	"bufio"
	"fmt"
	"io"
	"os/exec"
	"sync"
)

func SendVoice() error {
	cmd, stdin, stdout, stderr, err := Start()
	if err != nil {
		return fmt.Errorf("failed to start Python script: %v", err)
	}
	defer stdin.Close()

	var wg sync.WaitGroup

	// Start a goroutine to read and print Python stdout
	wg.Add(1)
	go func() {
		defer wg.Done()
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			fmt.Println("Python stdout:", scanner.Text())
		}
		if err := scanner.Err(); err != nil {
			fmt.Printf("Error reading Python stdout: %v\n", err)
		}
	}()

	// Start a goroutine to read and print Python stderr
	wg.Add(1)
	go func() {
		defer wg.Done()
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			fmt.Println("Python stderr:", scanner.Text())
		}
		if err := scanner.Err(); err != nil {
			fmt.Printf("Error reading Python stderr: %v\n", err)
		}
	}()

	// Wait for all output goroutines to finish
	wg.Wait()

	// Optionally wait for the Python script to finish
	err = cmd.Wait()
	if err != nil {
		return fmt.Errorf("Python script failed: %v", err)
	}

	return nil
}

// StartPythonScript starts the Python script for speech recognition.
func Start() (*exec.Cmd, io.WriteCloser, io.Reader, io.Reader, error) {
	cmd := exec.Command("python", "voice_recognition.py")

	// Create stdin pipe before starting the process
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("failed to get stdin pipe for Python: %v", err)
	}

	// Create stdout and stderr pipes
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("failed to get stdout pipe for Python: %v", err)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("failed to get stderr pipe for Python: %v", err)
	}

	// Start the Python script
	err = cmd.Start()
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("failed to start Python script: %v", err)
	}

	return cmd, stdin, stdout, stderr, nil
}
