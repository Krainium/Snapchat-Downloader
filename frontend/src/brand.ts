/** Everything that differs between the two builds lives here. */
export const BRAND = {
  name: "Snapchat Downloader",
  short: "Snapchat",
  headline: "Save Snapchat spotlights",
  lede:
    "Paste a spotlight or story link and the preview appears on its own. Download the original file in full quality. Nothing is stored on our side.",
  placeholder: "https://snapchat.com/t/…",
  accent: "#f7d417",
  linkPattern: /snapchat\.com\/[^\s]+/i,
  source: "https://github.com/Krainium/Snapchat-Downloader",
  steps: [
    { n: "01", t: "Copy the link", d: "Open the spotlight on Snapchat and tap share, then copy the link." },
    { n: "02", t: "Paste it here", d: "The preview loads by itself. No button to press, no account, no captcha." },
    { n: "03", t: "Save the file", d: "Download the original quality straight to your device." },
  ],
  features: [
    { t: "Original quality", d: "The file Snapchat serves, at the size it was published, never recompressed." },
    { t: "Spotlights and stories", d: "Public spotlight posts and public story links both work." },
    { t: "Nothing is stored", d: "Files stream straight through. No queue, no cache, no copy kept on the server." },
    { t: "No account", d: "No login, no extension, no app. Works the same on desktop and phone." },
  ],
  faq: [
    { q: "Which links work?", a: "Any public spotlight or story link from snapchat.com, including the short share links that start with snapchat.com/t/." },
    { q: "Can it fetch private snaps?", a: "No. Only content that is publicly viewable without logging in can be read, and that limit is deliberate." },
    { q: "What quality do I get?", a: "The file exactly as Snapchat stores it. There is no second rendition to choose from, so what you get is the original." },
    { q: "Do you keep my downloads?", a: "No. The server fetches the file and passes the bytes through as they arrive. Nothing is written to disk." },
  ],
};

/**
 * Palette tokens applied to :root at boot. Snap Yellow is extremely luminous,
 * so anything sitting on it takes black text rather than white, which is the
 * pairing Snapchat itself uses on its dark surfaces.
 */
export const THEME: Record<string, string> = {
  "--bg": "#0e0f10",
  "--ink": "#f4f4ef",
  "--dim": "#9d9d93",
  "--faint": "#6e6e66",
  "--line": "rgba(255, 252, 0, 0.10)",
  "--line-str": "rgba(255, 252, 0, 0.26)",
  "--panel": "rgba(24, 24, 20, 0.68)",
  "--panel-soft": "rgba(255, 252, 0, 0.045)",
  "--accent": "#fffc00",
  "--accent-2": "#fff67a",
  "--on-accent": "#101008",
  "--glow-1": "rgba(255, 252, 0, 0.20)",
  "--glow-2": "rgba(255, 178, 0, 0.14)",
  "--focus": "rgba(255, 252, 0, 0.55)",
};

/** Ghost outline, drawn to sit on the accent chip. */
export const ICON_PATH =
  "M12 3.2c2.7 0 4.1 1.9 4.1 4.4 0 .9-.1 1.6-.1 2 .5.3 1.2.2 1.7-.1.5.7.2 1.4-.4 1.7-.6.3-1.3.4-1.3.9 0 1 2.1 3.1 3.7 3.5.3.5-.1 1-.9 1.2-.6.2-1.3.2-1.5.5-.1.3-.1.7-.4.9-.5.2-1.7-.2-2.8 0-1 .2-1.7 1.5-3 1.5h-.6c-1.3 0-2-1.3-3-1.5-1.1-.2-2.3.2-2.8 0-.3-.2-.3-.6-.4-.9-.2-.3-.9-.3-1.5-.5-.8-.2-1.2-.7-.9-1.2 1.6-.4 3.7-2.5 3.7-3.5 0-.5-.7-.6-1.3-.9-.6-.3-.9-1-.4-1.7.5.3 1.2.4 1.7.1 0-.4-.1-1.1-.1-2 0-2.5 1.4-4.4 4.1-4.4z";
