// gemini-stt-module — Botmother tashqi modul: ovoz/audio'ni matnga (STT).
//
// Node turi:
//   - gemini-stt.Transcribe — action: audio (URL yoki platforma fayl UUID) ni
//     Google Gemini (AI Studio) orqali matnga aylantiradi. Natija `transcript`
//     state'iga yoziladi.
//
// Nega Gemini: AI Studio bepul tier beradi, multimodal (audio'ni to'g'ridan
// qabul qiladi — alohida Whisper kerak emas) va O'zbek tilini yaxshi tushunadi.
// Credential: Google AI Studio API key (https://aistudio.google.com).
package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	botmodule "github.com/BotSpace/botmodule-go"
)

const (
	moduleID = "gemini-stt"
	apiBase  = "https://generativelanguage.googleapis.com/v1beta/models"
)

var httpClient = &http.Client{Timeout: 120 * time.Second}

func main() {
	m := botmodule.New(moduleID, "Gemini STT")
	m.Version = "0.1.0"
	m.Docs = docs

	// Modul o'z credential turini e'lon qiladi — foydalanuvchi Google AI Studio
	// (Gemini) API key kiritadi. Key {slug}.* namespace bilan (platforma talabi).
	m.AddCredentialType(botmodule.CredentialType{
		Key:   "gemini-stt.apikey",
		Label: "Google AI Studio (Gemini)",
		Icon:  "mic",
		Color: "#1A73E8",
		Mode:  "apikey",
		Fields: []botmodule.CredentialField{
			{
				Name:        "api_key",
				Label:       "API Key",
				Type:        "password",
				Required:    true,
				Secret:      true,
				Placeholder: "AIza...",
			},
		},
	})

	m.AddNode(botmodule.Node{
		Type:        "gemini-stt.Transcribe",
		Title:       "Ovozni matnga (Gemini)",
		Description: "Audio/ovoz xabarni Gemini orqali matnga aylantiradi",
		Category:    "ai",
		Icon:        "mic",
		Color:       "ai-violet",
		Width:       200,
		Content: []botmodule.Field{
			{
				Type:           "credential",
				Key:            "api_credential",
				Label:          "Gemini API key",
				CredentialType: "gemini-stt.apikey",
				HelpText:       "https://aistudio.google.com — bepul kalit",
			},
			{
				Type:        "text",
				Key:         "audio",
				Label:       "Audio (URL yoki fayl)",
				Placeholder: "{{message.voice.file_url}}",
				HelpText:    "Ovoz/audio URL'i yoki platforma fayl UUID'si",
			},
			{
				Type:  "select",
				Key:   "language",
				Label: "Til",
				Options: []botmodule.SelectOption{
					{Value: "auto", Label: "Avtomatik aniqlash"},
					{Value: "uz", Label: "O'zbek"},
					{Value: "ru", Label: "Rus"},
					{Value: "en", Label: "Ingliz"},
				},
				Optional: true,
			},
			{
				Type:        "text",
				Key:         "model",
				Label:       "Model",
				Placeholder: "gemini-2.0-flash",
				HelpText:    "gemini-2.0-flash (bepul) yoki gemini-2.5-flash",
				Optional:    true,
			},
			{
				Type:        "text",
				Key:         "mime_type",
				Label:       "MIME turi",
				Placeholder: "audio/ogg",
				HelpText:    "Telegram ovozi = audio/ogg; mp3 = audio/mpeg; wav = audio/wav",
				Optional:    true,
			},
		},
		Defaults: map[string]any{
			"language": "auto",
			"model":    "gemini-2.0-flash",
		},
		Outputs: []botmodule.Output{
			{Name: "success", Label: "Matn", Variant: "success"},
			{Name: "error", Label: "Xato", Variant: "danger"},
		},
		ProducesState: []string{"transcript", "stt_error"},
		Execute:       executeTranscribe,
	})

	m.Serve(":8100")
}

func executeTranscribe(c *botmodule.ExecuteCtx) botmodule.Result {
	cred, ok := c.Credential("api_credential")
	if !ok {
		return errResult("Gemini API credential tanlanmagan")
	}
	apiKey := cred.Data["api_key"]
	if apiKey == "" {
		apiKey = cred.Data["token"]
	}
	if apiKey == "" {
		return errResult("API key bo'sh")
	}

	audioRef := strings.TrimSpace(c.String("audio"))
	if audioRef == "" {
		return errResult("audio bo'sh")
	}

	mimeType := strings.TrimSpace(c.String("mime_type"))
	if mimeType == "" {
		mimeType = "audio/ogg" // Telegram voice default
	}

	// Audio baytlarini olamiz: URL bo'lsa HTTP yuklaymiz, aks holda platforma
	// fayl UUID'si deb GetFile bilan o'qiymiz.
	audioBytes, err := loadAudio(c, audioRef)
	if err != nil {
		return errResult("audio olinmadi: " + err.Error())
	}
	if len(audioBytes) == 0 {
		return errResult("audio bo'sh (0 bayt)")
	}

	model := strings.TrimSpace(c.String("model"))
	if model == "" {
		model = "gemini-2.0-flash"
	}

	// Til ko'rsatilsa promptga qo'shamiz (auto = aniqlamaymiz).
	lang := strings.TrimSpace(c.String("language"))
	prompt := "Transcribe the following audio accurately. Return ONLY the transcribed text — no extra commentary, labels, quotes, or formatting."
	if lang != "" && lang != "auto" {
		prompt = fmt.Sprintf(
			"Transcribe the following audio accurately. The spoken language is %q. Return ONLY the transcribed text — no extra commentary, labels, quotes, or formatting.",
			lang,
		)
	}

	payload := map[string]any{
		"contents": []map[string]any{
			{"parts": []map[string]any{
				{"inline_data": map[string]any{
					"mime_type": mimeType,
					"data":      base64.StdEncoding.EncodeToString(audioBytes),
				}},
				{"text": prompt},
			}},
		},
		"generationConfig": map[string]any{"temperature": 0},
	}

	body, _ := json.Marshal(payload)
	url := fmt.Sprintf("%s/%s:generateContent?key=%s", apiBase, model, apiKey)
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return errResult("so'rov qurilmadi: " + err.Error())
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return errResult("Gemini so'rovi muvaffaqiyatsiz: " + err.Error())
	}
	defer resp.Body.Close()

	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return errResult(fmt.Sprintf("Gemini %d: %s", resp.StatusCode, truncate(string(raw), 300)))
	}

	var out struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return errResult("javob parse bo'lmadi: " + err.Error())
	}
	if out.Error.Message != "" {
		return errResult("Gemini: " + out.Error.Message)
	}

	var sb strings.Builder
	if len(out.Candidates) > 0 {
		for _, p := range out.Candidates[0].Content.Parts {
			sb.WriteString(p.Text)
		}
	}
	text := strings.TrimSpace(sb.String())
	if text == "" {
		return errResult("transkripsiya bo'sh (Gemini matn qaytarmadi)")
	}

	return botmodule.Result{
		ContextUpdates: map[string]any{
			"transcript": text,
			"stt_error":  "",
		},
		ExitOutput: "success",
	}
}

// loadAudio — audioRef http(s) URL bo'lsa HTTP GET qiladi, aks holda uni platforma
// fayl UUID'si deb GetFile bilan o'qiydi (modul fayl API'si engine'dan keladi).
func loadAudio(c *botmodule.ExecuteCtx, ref string) ([]byte, error) {
	if strings.HasPrefix(ref, "http://") || strings.HasPrefix(ref, "https://") {
		resp, err := httpClient.Get(ref)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("URL status %d", resp.StatusCode)
		}
		return io.ReadAll(resp.Body)
	}
	return c.GetFile(ref)
}

func errResult(msg string) botmodule.Result {
	return botmodule.Result{
		ContextUpdates: map[string]any{
			"transcript": "",
			"stt_error":  msg,
		},
		ExitOutput: "error",
	}
}

func truncate(s string, n int) string {
	if len(s) > n {
		return s[:n] + "..."
	}
	return s
}

const docs = `# Gemini STT (ovozni matnga)

Audio yoki ovozli xabarni [Google Gemini](https://aistudio.google.com) (AI Studio)
orqali matnga aylantiradi. Gemini multimodal — audio'ni to'g'ridan qabul qiladi,
alohida Whisper servisi kerak emas. AI Studio **bepul tier** beradi va O'zbek
tilini yaxshi tushunadi.

## Node turi

### ` + "`gemini-stt.Transcribe`" + ` (action, AI)

| Field | Tavsif |
|---|---|
| **api_credential** | Google AI Studio (Gemini) API key. https://aistudio.google.com |
| **audio** | Ovoz/audio URL'i yoki platforma fayl UUID'si (` + "`{{message.voice.file_url}}`" + `) |
| **language** | Til: avtomatik / o'zbek / rus / ingliz (ixtiyoriy) |
| **model** | ` + "`gemini-2.0-flash`" + ` (bepul) yoki ` + "`gemini-2.5-flash`" + ` (ixtiyoriy) |
| **mime_type** | Audio formati: ` + "`audio/ogg`" + ` (Telegram ovozi), ` + "`audio/mpeg`" + `, ` + "`audio/wav`" + ` (ixtiyoriy) |

**Chiqish state'lari:**

- ` + "`transcript`" + ` — tanib olingan matn
- ` + "`stt_error`" + ` — xato matni (muvaffaqiyatda bo'sh)

**Chiqish edge'lari:** ` + "`success`" + ` (Matn) / ` + "`error`" + ` (Xato)

## Misol flow

` + "```" + `
Ovoz kelganda (trigger)
  → Ovozni matnga (Gemini)  (audio: {{message.voice.file_url}})
  → Matn yuborish ({{transcript}})
` + "```" + `

## Audio manbasi

` + "`audio`" + ` maydoni ikki xil qiymatni qabul qiladi:

- **URL** (` + "`http(s)://`" + `) — modul HTTP orqali yuklab oladi.
- **Fayl UUID** — platformaga saqlangan faylni ` + "`GetFile`" + ` orqali o'qiydi.

> Telegram ovozli xabari uchun yuklab olinadigan URL (yoki avval faylni saqlash)
> kerak. ` + "`mime_type`" + ` ni audio formatiga moslang (voice = ` + "`audio/ogg`" + `).

## Cheklovlar

Audio inline yuboriladi (Gemini ` + "`inline_data`" + `) — so'rov ~20MB gacha.
Uzun audiolar uchun Gemini File API kerak (keyingi bosqich).
`
