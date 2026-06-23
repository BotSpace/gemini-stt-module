# gemini-stt-module

Botmother tashqi moduli — ovoz/audio xabarni Google Gemini (AI Studio) orqali
matnga aylantiradi (STT).

## Nega Gemini
- **Bepul tier** (https://aistudio.google.com — karta'siz kalit)
- **Multimodal** — audio'ni to'g'ridan qabul qiladi (alohida Whisper kerak emas)
- **O'zbek tili** yaxshi qo'llab-quvvatlanadi

## Node
`gemini-stt.Transcribe` (action, AI) — audio (URL yoki fayl UUID) → `transcript` state.

To'liq tavsif: `describe()` RPC / SDK.md. Credential: `gemini-stt.apikey` (AI Studio API key).

## Lokal test
```bash
go run .
curl http://localhost:8100/health
```

## Build
```bash
docker build -t gemini-stt .
docker run -p 8100:8100 gemini-stt
```

SDK: `github.com/BotSpace/botmodule-go`.
