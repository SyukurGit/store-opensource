package silvex

import (
	"strings"
)

// KnownAILogo represents a preset icon with human readable title and icon URL
type KnownAILogo struct {
	Title string `json:"title"`
	URL   string `json:"url"`
}

// GetPresetAILogos returns popular AI application logos available for 1-click selection
func GetPresetAILogos() []KnownAILogo {
	return []KnownAILogo{
		{Title: "ChatGPT / OpenAI", URL: "https://api.iconify.design/simple-icons:openai.svg?color=%2310a37f"},
		{Title: "Claude (Anthropic)", URL: "https://api.iconify.design/simple-icons:anthropic.svg?color=%23d97706"},
		{Title: "Midjourney", URL: "https://api.iconify.design/simple-icons:midjourney.svg?color=%23ffffff"},
		{Title: "Canva Pro", URL: "https://api.iconify.design/simple-icons:canva.svg?color=%2300c4cc"},
		{Title: "Cursor AI", URL: "https://api.iconify.design/simple-icons:cursor.svg?color=%2338bdf8"},
		{Title: "Perplexity Pro", URL: "https://api.iconify.design/simple-icons:perplexity.svg?color=%2320b2aa"},
		{Title: "GitHub Copilot", URL: "https://api.iconify.design/simple-icons:githubcopilot.svg?color=%23ffffff"},
		{Title: "Google Gemini", URL: "https://api.iconify.design/simple-icons:googlegemini.svg?color=%234285f4"},
		{Title: "ElevenLabs", URL: "https://api.iconify.design/simple-icons:elevenlabs.svg?color=%23ffffff"},
		{Title: "Suno AI", URL: "https://api.iconify.design/simple-icons:suno.svg?color=%23f43f5e"},
		{Title: "Runway Gen", URL: "https://api.iconify.design/simple-icons:runway.svg?color=%236366f1"},
		{Title: "DeepL Pro", URL: "https://api.iconify.design/simple-icons:deepl.svg?color=%230f2b46"},
		{Title: "Grammarly", URL: "https://api.iconify.design/simple-icons:grammarly.svg?color=%2315c39a"},
		{Title: "CapCut Pro", URL: "https://api.iconify.design/simple-icons:capcut.svg?color=%23ffffff"},
		{Title: "Netflix", URL: "https://api.iconify.design/simple-icons:netflix.svg?color=%23e50914"},
		{Title: "Spotify", URL: "https://api.iconify.design/simple-icons:spotify.svg?color=%231ed760"},
		{Title: "YouTube Premium", URL: "https://api.iconify.design/simple-icons:youtube.svg?color=%23ff0000"},
	}
}

// ResolveAILogo resolves a product's logo by custom logo, keyword matching, or default fallback
func ResolveAILogo(name string, customLogo string) string {
	if strings.TrimSpace(customLogo) != "" {
		return strings.TrimSpace(customLogo)
	}

	lower := strings.ToLower(name)
	switch {
	case strings.Contains(lower, "chatgpt") || strings.Contains(lower, "openai") || strings.Contains(lower, "gpt"):
		return "https://api.iconify.design/simple-icons:openai.svg?color=%2310a37f"
	case strings.Contains(lower, "claude") || strings.Contains(lower, "anthropic"):
		return "https://api.iconify.design/simple-icons:anthropic.svg?color=%23d97706"
	case strings.Contains(lower, "midjourney"):
		return "https://api.iconify.design/simple-icons:midjourney.svg?color=%23ffffff"
	case strings.Contains(lower, "canva"):
		return "https://api.iconify.design/simple-icons:canva.svg?color=%2300c4cc"
	case strings.Contains(lower, "cursor"):
		return "https://api.iconify.design/simple-icons:cursor.svg?color=%2338bdf8"
	case strings.Contains(lower, "perplexity"):
		return "https://api.iconify.design/simple-icons:perplexity.svg?color=%2320b2aa"
	case strings.Contains(lower, "copilot") || strings.Contains(lower, "github"):
		return "https://api.iconify.design/simple-icons:githubcopilot.svg?color=%23ffffff"
	case strings.Contains(lower, "gemini"):
		return "https://api.iconify.design/simple-icons:googlegemini.svg?color=%234285f4"
	case strings.Contains(lower, "elevenlabs"):
		return "https://api.iconify.design/simple-icons:elevenlabs.svg?color=%23ffffff"
	case strings.Contains(lower, "suno"):
		return "https://api.iconify.design/simple-icons:suno.svg?color=%23f43f5e"
	case strings.Contains(lower, "runway"):
		return "https://api.iconify.design/simple-icons:runway.svg?color=%236366f1"
	case strings.Contains(lower, "deepl"):
		return "https://api.iconify.design/simple-icons:deepl.svg?color=%230f2b46"
	case strings.Contains(lower, "grammarly"):
		return "https://api.iconify.design/simple-icons:grammarly.svg?color=%2315c39a"
	case strings.Contains(lower, "capcut"):
		return "https://api.iconify.design/simple-icons:capcut.svg?color=%23ffffff"
	case strings.Contains(lower, "netflix"):
		return "https://api.iconify.design/simple-icons:netflix.svg?color=%23e50914"
	case strings.Contains(lower, "spotify"):
		return "https://api.iconify.design/simple-icons:spotify.svg?color=%231ed760"
	case strings.Contains(lower, "youtube"):
		return "https://api.iconify.design/simple-icons:youtube.svg?color=%23ff0000"
	default:
		return "/logo/digimest_petak.png"
	}
}
