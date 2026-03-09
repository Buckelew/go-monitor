package discord

import (
	"github.com/bwmarrin/discordgo"
)

const (
	colorBlue  = 0x3498db
	colorGreen = 0x2ecc71
	colorRed   = 0xe74c3c
)

func NewProductEmbed(title, url, imageURL string) *discordgo.MessageEmbed {
	embed := &discordgo.MessageEmbed{
		Title: title,
		URL:   url,
		Color: colorBlue,
		Footer: &discordgo.MessageEmbedFooter{
			Text: "New Product",
		},
	}
	if imageURL != "" {
		embed.Thumbnail = &discordgo.MessageEmbedThumbnail{URL: imageURL}
	}
	return embed
}

func RestockEmbed(title, url, imageURL string) *discordgo.MessageEmbed {
	embed := &discordgo.MessageEmbed{
		Title: title,
		URL:   url,
		Color: colorGreen,
		Footer: &discordgo.MessageEmbedFooter{
			Text: "Restock",
		},
	}
	if imageURL != "" {
		embed.Thumbnail = &discordgo.MessageEmbedThumbnail{URL: imageURL}
	}
	return embed
}

func DelistedEmbed(title, url string) *discordgo.MessageEmbed {
	return &discordgo.MessageEmbed{
		Title: title,
		URL:   url,
		Color: colorRed,
		Footer: &discordgo.MessageEmbedFooter{
			Text: "Delisted",
		},
	}
}
