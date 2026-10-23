# Frontend Preferences

- Prioritizes mobile UX/responsiveness; asks for mobile fixes explicitly scoped to mobile ("mobile aja") while leaving desktop and dark mode untouched. Confidence: 0.85
- Expects responsive work verified against real mobile viewports (e.g. iPhone 12 / 390px; range 320px–430px) with no horizontal overflow. Confidence: 0.8
- Wants UI polished for both dark and light modes, keeping theme parity (e.g. light mode needs clear borders so white components don't blend into light backgrounds). Confidence: 0.75
- Prefers modern, clean UI shapes — e.g. rounded/pill badges over square ones — while preserving the existing brand colors, layout, and structure (no full redesign). Confidence: 0.65
- Expects public-facing UI changes mirrored across locales: the Indonesian pages and the `/en` versions must stay in sync (translated labels, not just structural copies). Confidence: 0.8
- Wants third-party brand icons (WhatsApp, TikTok, Threads) rendered in each platform's official brand colors/theme rather than generic monochrome icons ("sesuai dengan teme logonya"). Confidence: 0.8
- In the admin panel, prefers a dedicated, separate menu/page per feature concern over cramming new fields into existing settings pages ("satu menu terpisah"); existing settings should just link to it. Confidence: 0.8
- Preserves an element's original meaning and iconography: do NOT replace a generic/support mark (e.g. a headset "Help CS" icon) with a vendor's brand logo — vendor logos belong only on that vendor's own row/button. Confidence: 0.85
- Keeps distinct concerns in separate, clearly-labeled groups instead of merging them — e.g. support channels (WhatsApp/Telegram under "Help CS") must not land inside the "Ikuti Kami / Follow Us" social section. Confidence: 0.8
- When several channels serve the same purpose, expects an explicit hierarchy (primary vs secondary — e.g. WhatsApp primary, Telegram additional) surfaced in both the UI labels/badges and the admin form. Confidence: 0.8
- Admin forms must expose a field for every configurable channel/target, including the secondary/fallback one (e.g. a Telegram direct field next to WhatsApp) — not just the primary channel. Confidence: 0.75
