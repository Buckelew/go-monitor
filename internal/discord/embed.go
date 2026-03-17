package discord

import (
	"time"

	"github.com/bwmarrin/discordgo"
)

const (
	colorBlue  = 0x0099ff
	colorGreen = 0x2ecc71
	colorRed   = 0xe74c3c
	embedFooter = "@buckelew"
)

func NewProductEmbed(title, store, url, imageURL string) *discordgo.MessageEmbed {
	embed := &discordgo.MessageEmbed{
		Title:     title,
		URL:       url,
		Color:     colorBlue,
		Author:    &discordgo.MessageEmbedAuthor{Name: store, URL: url},
		Footer:    &discordgo.MessageEmbedFooter{Text: embedFooter},
		Timestamp: time.Now().Format(time.RFC3339),
	}
	if imageURL != "" {
		embed.Thumbnail = &discordgo.MessageEmbedThumbnail{URL: imageURL}
	}
	return embed
}

func RestockEmbed(title, store, url, imageURL string) *discordgo.MessageEmbed {
	embed := &discordgo.MessageEmbed{
		Title:     title + " Restocked",
		URL:       url,
		Color:     colorGreen,
		Author:    &discordgo.MessageEmbedAuthor{Name: store, URL: url},
		Footer:    &discordgo.MessageEmbedFooter{Text: embedFooter},
		Timestamp: time.Now().Format(time.RFC3339),
	}
	if imageURL != "" {
		embed.Thumbnail = &discordgo.MessageEmbedThumbnail{URL: imageURL}
	}
	return embed
}

func DelistedEmbed(title, store, url string) *discordgo.MessageEmbed {
	return &discordgo.MessageEmbed{
		Title:     title + " Delisted",
		URL:       url,
		Color:     colorRed,
		Author:    &discordgo.MessageEmbedAuthor{Name: store, URL: url},
		Footer:    &discordgo.MessageEmbedFooter{Text: embedFooter},
		Timestamp: time.Now().Format(time.RFC3339),
	}
}

func PasswordUpEmbed(store string) *discordgo.MessageEmbed {
	return &discordgo.MessageEmbed{
		Title:       "Password page up",
		Description: store,
		Color:       colorRed,
		Footer:      &discordgo.MessageEmbedFooter{Text: embedFooter},
		Timestamp:   time.Now().Format(time.RFC3339),
	}
}

func PasswordDownEmbed(store string) *discordgo.MessageEmbed {
	return &discordgo.MessageEmbed{
		Title:       "Password page down",
		Description: store,
		Color:       colorGreen,
		Footer:      &discordgo.MessageEmbedFooter{Text: embedFooter},
		Timestamp:   time.Now().Format(time.RFC3339),
	}
}
