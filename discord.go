package main

import (
	"fmt"
	"io"

	"github.com/bwmarrin/discordgo"
)

func HandleVoice(c chan *discordgo.Packet, stdin io.WriteCloser) error {
	for p := range c {
		_, err := stdin.Write(p.Opus)
		if err != nil {
			return fmt.Errorf("failed to send audio data: %v", err)
		}
		fmt.Println("Audio data sent.")
	}
	return nil
}
