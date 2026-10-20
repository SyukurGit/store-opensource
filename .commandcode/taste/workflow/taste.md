# Workflow Preferences

- Demands deep, evidence-based investigation BEFORE any code change, and explicitly forbids editing based on assumptions ("jgn asal", "jangan langsung ubah kode berdasarkan asumsi", "pahami dulu"). Confidence: 0.9
- When asking for analysis, expects strictly read-only work — explicitly says not to edit anything ("jgn edit apapaun") and wants confirmation that zero files changed. Confidence: 0.85
- Wants end-to-end tracing of a feature (route → middleware/handler → service → struct/DB → frontend API client → UI) plus an explicit impact analysis listing affected backend and `/web` files and side effects before editing. Confidence: 0.85
- Requires surgical, isolated changes with backward compatibility; an explicit goal is "zero regression" — never break features or flows that already work ("jgn rusak yg lain"). Confidence: 0.9
- Insists on actually implementing the fix, not stopping at analysis, recommendations, or mockups ("Jangan berhenti pada saran, analisis, atau mockup. Implementasikan"). Confidence: 0.85
- Requires a build/compile check before reporting done — e.g. `go build ./...` at the Go root (and a type/build check for the frontend). Confidence: 0.85
- Prefers an analysis/plan first, then an explicit go-ahead ("eksekusi") before changes are made. Confidence: 0.75
- When a change spans tightly-coupled backend and frontend, expects both sides synced in the same change (contract changes mirrored to `/web`). Confidence: 0.8
