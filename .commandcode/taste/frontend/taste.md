# Frontend Preferences

- Prioritizes mobile UX/responsiveness; asks for mobile fixes explicitly scoped to mobile ("mobile aja") while leaving desktop and dark mode untouched. Confidence: 0.85
- Expects responsive work verified against real mobile viewports (e.g. iPhone 12 / 390px; range 320px–430px) with no horizontal overflow. Confidence: 0.8
- Wants UI polished for both dark and light modes, keeping theme parity (e.g. light mode needs clear borders so white components don't blend into light backgrounds). Confidence: 0.75
- Prefers modern, clean UI shapes — e.g. rounded/pill badges over square ones — while preserving the existing brand colors, layout, and structure (no full redesign). Confidence: 0.65
- Expects public-facing UI changes mirrored across locales: the Indonesian pages and the `/en` versions must stay in sync (translated labels, not just structural copies). Repeats "cek semua" — ID, EN, mobile, desktop, dark and light — before calling a UI change done. Confidence: 0.85
- Wants third-party brand icons (WhatsApp, TikTok, Threads) rendered in each platform's official brand colors/theme rather than generic monochrome icons ("sesuai dengan teme logonya"). Confidence: 0.8
- In the admin panel, prefers a dedicated, separate menu/page per feature concern over cramming new fields into existing settings pages ("satu menu terpisah"); existing settings should just link to it. Confidence: 0.8
- Preserves an element's original meaning and iconography: do NOT replace a generic/support mark (e.g. a headset "Help CS" icon) with a vendor's brand logo — vendor logos belong only on that vendor's own row/button. Confidence: 0.85
- Keeps distinct concerns in separate, clearly-labeled groups instead of merging them — e.g. support channels (WhatsApp/Telegram under "Help CS") must not land inside the "Ikuti Kami / Follow Us" social section. Confidence: 0.8
- When several channels serve the same purpose, a hierarchy (primary vs secondary) may exist, but do NOT surface it as visible customer-facing labels/badges like "UTAMA"/"TAMBAHAN"/"PRIMARY"/"SECONDARY" — order and placement should imply it; such badges read as clutter. Confidence: 0.9
- Rejects anything that looks like an AI-generated template ("keliatan bantuan ai"); wants professional, deliberate UX where every visible label earns its place. Redundant status chips, kickers, or explanatory micro-copy should be removed, not added. Confidence: 0.85
- Every clickable row/card must look clickable: expects a clear affordance such as a trailing chevron/arrow, especially on mobile, and objects to elements that merely *look* actionable ("seolah bisa di klik") when they are not. Confidence: 0.85
- Action buttons that carry the site's own meaning (floating Help CS / chat pop-up) should use the site's theme accent color (e.g. blue matching the web theme), not a vendor brand color; vendor brand colors stay on that vendor's own icon inside its row. Confidence: 0.8
- Admin forms must expose a field for every configurable channel/target, including the secondary/fallback one (e.g. a Telegram direct field next to WhatsApp) — not just the primary channel. Confidence: 0.75
