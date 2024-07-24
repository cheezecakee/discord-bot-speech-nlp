package nlp

import (
	"fmt"

	"github.com/bwmarrin/discordgo"
)

func HandleVoice(c chan *discordgo.Packet, pipe chan []byte) error {
	for p := range c {
		pipe <- p.Opus
		fmt.Println("Audio data sent.")
	}
	return nil
}
