package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"net/url"

	"github.com/bwmarrin/discordgo"
	"github.com/joho/godotenv"
)

const (
	pageSize = 10
	apiURL   = "http://127.0.0.1:5000"
)

type Player struct {
	Username   string `json:"username"`
	ProfileURL string `json:"profile_url"`
	Playtime   int    `json:"playtime"`
	HeaderURL  string `json:"header_url"` // Added to hold header image
}

type SearchResult struct {
	ImgIconURL string   `json:"img_icon_url"`
	HeaderURL  string   `json:"header_url"`
	Users      []Player `json:"users"`
}

func main() {
	if err := godotenv.Load(); err != nil {
		fmt.Println("Error loading .env file:", err)
		return
	}

	Token := os.Getenv("DISCORD_BOT_TOKEN")
	if Token == "" {
		fmt.Println("Error: DISCORD_BOT_TOKEN not set")
		return
	}

	dg, err := discordgo.New("Bot " + Token)
	if err != nil {
		fmt.Println("Error creating Discord session,", err)
		return
	}

	dg.AddHandler(messageCreate)
	dg.AddHandler(interactionCreate)

	if err := dg.Open(); err != nil {
		fmt.Println("Error opening connection,", err)
		return
	}
	fmt.Println("Bot is now running. Press CTRL+C to exit.")

	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
	<-sc
	dg.Close()
}

func messageCreate(s *discordgo.Session, m *discordgo.MessageCreate) {
	if m.Author.ID == s.State.User.ID || !strings.HasPrefix(m.Content, "!") {
		return
	}

	parts := strings.Fields(m.Content)
	command := strings.TrimPrefix(parts[0], "!")
	args := parts[1:]

	switch command {
	case "ping":
		handlePing(s, m)
	case "register":
		handleRegister(s, m, args)
	case "games":
		handleGames(s, m, args)
	case "search":
		handleSearch(s, m, args)
	}
}

func handlePing(s *discordgo.Session, m *discordgo.MessageCreate) {
	s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("Pong, %s!", m.Author.Mention()))
}

func handleRegister(s *discordgo.Session, m *discordgo.MessageCreate, args []string) {
	if len(args) != 1 {
		s.ChannelMessageSend(m.ChannelID, "Usage: `!register <steamid>`")
		return
	}

	steamid := args[0]
	payload := fmt.Sprintf(`{"steamid":"%s"}`, steamid)
	resp, err := http.Post(apiURL+"/register", "application/json", strings.NewReader(payload))
	if err != nil {
		s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("Error calling API: %v", err))
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != 201 {
		body, _ := io.ReadAll(resp.Body)
		s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("API error: %s", string(body)))
		return
	}

	var account map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&account); err != nil {
		s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("Error parsing JSON: %v", err))
		return
	}

	embed := &discordgo.MessageEmbed{
		Title: account["steamid"].(string),
		URL:   account["profile_url"].(string),
		Color: 0x33ccbb,
		Thumbnail: &discordgo.MessageEmbedThumbnail{
			URL: account["avatar_url"].(string),
		},
		Footer: &discordgo.MessageEmbedFooter{
			Text: "Registration successful",
		},
	}
	s.ChannelMessageSendEmbed(m.ChannelID, embed)
}

func handleGames(s *discordgo.Session, m *discordgo.MessageCreate, args []string) {
	if len(args) != 1 {
		s.ChannelMessageSend(m.ChannelID, "Usage: `!games <username>`")
		return
	}
	username := args[0]
	apiResp, err := http.Get(fmt.Sprintf("%s/user/%s", apiURL, username))
	if err != nil {
		s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("Error fetching user: %v", err))
		return
	}
	defer apiResp.Body.Close()

	if apiResp.StatusCode != 200 {
		body, _ := io.ReadAll(apiResp.Body)
		s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("API error: %s", string(body)))
		return
	}

	var account struct {
		ProfileURL string                            `json:"profile_url"`
		AvatarURL  string                            `json:"avatar_url"`
		Games      map[string]map[string]interface{} `json:"games"`
	}
	if err := json.NewDecoder(apiResp.Body).Decode(&account); err != nil {
		s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("Error parsing JSON: %v", err))
		return
	}

	players := []Player{}
	for name, g := range account.Games {
		playtime := int(g["playtime_forever"].(float64)) / 60
		headerURL := ""
		if v, ok := g["header_url"].(string); ok {
			headerURL = v
		}
		players = append(players, Player{
			Username:   name,
			ProfileURL: g["store_url"].(string),
			Playtime:   playtime,
			HeaderURL:  headerURL,
		})
	}

	sort.Slice(players, func(i, j int) bool { return players[i].Playtime > players[j].Playtime })

	sendPaginatedEmbed(s, m.ChannelID, username, account.AvatarURL, players, "games", 0)
}

func handleSearch(s *discordgo.Session, m *discordgo.MessageCreate, args []string) {
	if len(args) == 0 {
		s.ChannelMessageSend(m.ChannelID, "Usage: `!search <game name>`")
		return
	}

	gameName := strings.Join(args, " ")
	encodedGame := url.QueryEscape(gameName)
	fullURL := fmt.Sprintf("%s/search?game=%s", apiURL, encodedGame)

	resp, err := http.Get(fullURL)
	if err != nil {
		s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("Error calling API: %v", err))
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("API error (status %d): %s", resp.StatusCode, string(body)))
		return
	}

	var result SearchResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("Error parsing JSON: %v", err))
		return
	}

	if len(result.Users) == 0 {
		s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("No users found for **%s**", gameName))
		return
	}

	sendPaginatedEmbed(s, m.ChannelID, gameName, result.ImgIconURL, result.Users, "search", 0)
}

func sendPaginatedEmbed(s *discordgo.Session, channelID, title, thumb string, players []Player, prefix string, page int) {
	totalPages := (len(players) + pageSize - 1) / pageSize
	if page < 0 {
		page = 0
	} else if page >= totalPages {
		page = totalPages - 1
	}

	start := page * pageSize
	end := start + pageSize
	if end > len(players) {
		end = len(players)
	}

	var sb strings.Builder
	for _, p := range players[start:end] {
		playtimeMsg := "Less than one hour"
		if p.Playtime == 1 {
			playtimeMsg = "1 hour"
		} else if p.Playtime > 1 {
			playtimeMsg = fmt.Sprintf("%d hours", p.Playtime)
		}
		sb.WriteString(fmt.Sprintf("[**%s**](%s) — %s\n", p.Username, p.ProfileURL, playtimeMsg))
	}

	imageURL := ""
	if len(players[start:end]) > 0 {
		imageURL = players[start].HeaderURL // first game's header
	}

	embed := &discordgo.MessageEmbed{
		Title:       fmt.Sprintf("%s — Page %d/%d", title, page+1, totalPages),
		Description: sb.String(),
		Color:       0x33ccbb,
		Thumbnail:   &discordgo.MessageEmbedThumbnail{URL: thumb},
		Image:       &discordgo.MessageEmbedImage{URL: imageURL},
		Footer:      &discordgo.MessageEmbedFooter{Text: fmt.Sprintf("Total items: %d", len(players))},
	}

	row := discordgo.ActionsRow{
		Components: []discordgo.MessageComponent{
			discordgo.Button{
				Label:    "◀ Previous",
				Style:    discordgo.PrimaryButton,
				CustomID: fmt.Sprintf("%s|prev|%s|%d", prefix, strings.ReplaceAll(title, "|", ""), page),
				Disabled: page == 0,
			},
			discordgo.Button{
				Label:    "Next ▶",
				Style:    discordgo.PrimaryButton,
				CustomID: fmt.Sprintf("%s|next|%s|%d", prefix, strings.ReplaceAll(title, "|", ""), page),
				Disabled: page >= totalPages-1,
			},
		},
	}

	s.ChannelMessageSendComplex(channelID, &discordgo.MessageSend{
		Embed:      embed,
		Components: []discordgo.MessageComponent{row},
	})
}

func editPaginatedEmbed(s *discordgo.Session, i *discordgo.InteractionCreate, title, thumb string, players []Player, prefix string, page int) {
	totalPages := (len(players) + pageSize - 1) / pageSize
	if page < 0 {
		page = 0
	} else if page >= totalPages {
		page = totalPages - 1
	}

	start := page * pageSize
	end := start + pageSize
	if end > len(players) {
		end = len(players)
	}

	var sb strings.Builder
	for _, p := range players[start:end] {
		playtimeMsg := "Less than one hour"
		if p.Playtime == 1 {
			playtimeMsg = "1 hour"
		} else if p.Playtime > 1 {
			playtimeMsg = fmt.Sprintf("%d hours", p.Playtime)
		}
		sb.WriteString(fmt.Sprintf("[**%s**](%s) — %s\n", p.Username, p.ProfileURL, playtimeMsg))
	}

	imageURL := ""
	if len(players[start:end]) > 0 {
		imageURL = players[start].HeaderURL
	}

	embed := &discordgo.MessageEmbed{
		Title:       fmt.Sprintf("%s — Page %d/%d", title, page+1, totalPages),
		Description: sb.String(),
		Color:       0x33ccbb,
		Thumbnail:   &discordgo.MessageEmbedThumbnail{URL: thumb},
		Image:       &discordgo.MessageEmbedImage{URL: imageURL},
		Footer:      &discordgo.MessageEmbedFooter{Text: fmt.Sprintf("Total items: %d", len(players))},
	}

	row := discordgo.ActionsRow{
		Components: []discordgo.MessageComponent{
			discordgo.Button{
				Label:    "◀ Previous",
				Style:    discordgo.PrimaryButton,
				CustomID: fmt.Sprintf("%s|prev|%s|%d", prefix, strings.ReplaceAll(title, "|", ""), page),
				Disabled: page == 0,
			},
			discordgo.Button{
				Label:    "Next ▶",
				Style:    discordgo.PrimaryButton,
				CustomID: fmt.Sprintf("%s|next|%s|%d", prefix, strings.ReplaceAll(title, "|", ""), page),
				Disabled: page >= totalPages-1,
			},
		},
	}

	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseUpdateMessage,
		Data: &discordgo.InteractionResponseData{
			Embeds:     []*discordgo.MessageEmbed{embed},
			Components: []discordgo.MessageComponent{row},
		},
	})
}

func interactionCreate(s *discordgo.Session, i *discordgo.InteractionCreate) {
	if i.Type != discordgo.InteractionMessageComponent {
		return
	}

	data := i.MessageComponentData()
	parts := strings.Split(data.CustomID, "|")
	if len(parts) < 4 {
		fmt.Println("Malformed customID:", data.CustomID)
		return
	}

	prefix, action, title := parts[0], parts[1], parts[2]
	page, _ := strconv.Atoi(parts[3])

	var players []Player
	var thumb string

	switch prefix {
	case "games":
		apiResp, err := http.Get(fmt.Sprintf("%s/user/%s", apiURL, title))
		if err != nil || apiResp.StatusCode != 200 {
			fmt.Println("Error fetching user for pagination")
			return
		}
		defer apiResp.Body.Close()

		var account struct {
			AvatarURL string                            `json:"avatar_url"`
			Games     map[string]map[string]interface{} `json:"games"`
		}
		json.NewDecoder(apiResp.Body).Decode(&account)
		thumb = account.AvatarURL
		for name, g := range account.Games {
			playtime := int(g["playtime_forever"].(float64)) / 60
			headerURL := ""
			if v, ok := g["header_url"].(string); ok {
				headerURL = v
			}
			players = append(players, Player{
				Username:   name,
				ProfileURL: g["store_url"].(string),
				Playtime:   playtime,
				HeaderURL:  headerURL,
			})
		}

	case "search":
		apiResp, err := http.Get(fmt.Sprintf("%s/search?game=%s", apiURL, title))
		if err != nil || apiResp.StatusCode != 200 {
			fmt.Println("Error fetching search for pagination")
			return
		}
		defer apiResp.Body.Close()

		var result SearchResult
		json.NewDecoder(apiResp.Body).Decode(&result)
		players = result.Users
		thumb = result.ImgIconURL
	}

	sort.Slice(players, func(i, j int) bool { return players[i].Playtime > players[j].Playtime })

	if action == "next" {
		page++
	} else if action == "prev" {
		page--
	}

	editPaginatedEmbed(s, i, title, thumb, players, prefix, page)
}
