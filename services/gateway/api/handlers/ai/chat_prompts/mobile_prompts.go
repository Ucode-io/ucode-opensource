package chat_prompts

// PromptCapacitorMobileAddendum is appended when project_type == "mobile".
// Mobile and webapp share the same React/Vite generator; this block adds the
// installable Capacitor contract without changing the generated UI stack.
const PromptCapacitorMobileAddendum = `
====================================
INSTALLABLE MOBILE APP — CAPACITOR CONTRACT
====================================
This project is an installable iOS/Android app built from the generated React + Vite webapp with Capacitor.
It is NOT React Native and does NOT use Expo. Keep using the webapp architecture, DOM, Tailwind CSS,
React Router, src/pages/* and the existing web API utilities.

The backend injects package.json Capacitor dependencies, index.html, capacitor.config.ts, src/lib/capacitor.ts,
mobile.capabilities.json, and approved native capability wrappers.
NEVER emit or replace those files. NEVER emit ios/ or android/ directories; a trusted build worker creates
them later with npx cap add and updates them with npx cap sync. The backend also normalizes the generated
BrowserRouter to HashRouter so routes survive native WebView reloads.
The approved '@/lib/capacitor' exports are: isNativePlatform, initializeNativeShell, hapticTap,
and listenForAndroidBackButton.

MOBILE RUNTIME REQUIREMENTS:
- Keep the phone-first shell, fixed bottom tabs, single-column screens, and touch targets >= 44px.
- Respect safe areas with CSS env(safe-area-inset-top/right/bottom/left). The sticky Header must reserve the
  top inset and have a solid opaque background; it must never float transparently over the hero/content.
- Do not rely on hover, desktop-only keyboard shortcuts, or wide data tables.
- Use browser-compatible APIs by default. Import from '@/lib/capacitor' only when native behavior is needed.
- Native plugin calls must keep a browser fallback so the microfrontend preview continues to work.
- Do not import React Native, Expo, @expo/*, or arbitrary Capacitor plugins.
- Use only the approved wrapper imports listed in the capability block below. Never import plugins directly.
`
