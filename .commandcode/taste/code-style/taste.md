# Code Style & Editing Preferences

- Follow the repo's existing design patterns, coding style, and error-handling conventions; no blind refactoring, and don't rename globals or delete helper functions outside the requested scope. Confidence: 0.85
- Treat API contracts as sacred: never change Go `json:"..."` struct tags, response types, status codes, or endpoint URLs without checking and syncing every `/web` caller. Confidence: 0.9
- Never touch network/proxy/CORS or cookie/session settings (`SameSite`, `Secure`, `Domain`), proxy-IP header reading, or `.env` without explicit approval. Confidence: 0.85
- Keep configuration and secret keys in environment variables (`.env`) rather than hardcoded. Confidence: 0.7
- Avoid "band-aid" fixes (shrinking font sizes, adding `overflow: hidden`, patches) when a root-cause structural fix is possible. Confidence: 0.8
