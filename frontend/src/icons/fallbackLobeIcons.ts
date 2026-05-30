const fallbackIcons: Record<string, string> = {
  custom: `<svg viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg">
    <path d="M5 8h10.5a3.5 3.5 0 010 7H8" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" />
    <path d="M8 5L5 8l3 3M16 12l3 3-3 3" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round" />
  </svg>`,
  aicoding: `<svg viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg">
    <rect x="4" y="4" width="16" height="16" rx="5" stroke="currentColor" stroke-width="1.6" />
    <path d="M9 15l3-6 3 6" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round" />
    <circle cx="12" cy="15" r="1.2" fill="currentColor" />
  </svg>`,
  kimi: `<svg viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg">
    <circle cx="8" cy="12" r="3.2" stroke="currentColor" stroke-width="1.5" />
    <circle cx="16" cy="12" r="3.2" stroke="currentColor" stroke-width="1.5" />
    <path d="M5 8l3.5-3.5M19 8l-3.5-3.5M5 16l3.5 3.5M19 16l-3.5 3.5" stroke="currentColor" stroke-width="1.3" stroke-linecap="round" />
  </svg>`,
  deepseek: `<svg viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg">
    <path d="M4 12l8.5-8.5L21 12l-8.5 8.5L4 12z" stroke="currentColor" stroke-width="1.4" stroke-linejoin="round" />
    <path d="M8.5 12L12 8.5 15.5 12 12 15.5 8.5 12z" fill="currentColor" />
  </svg>`,
}

export default fallbackIcons
