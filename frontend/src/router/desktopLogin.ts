import { ref } from 'vue'

export const desktopLoginTicket = ref<string | null>(null)

export function captureDesktopLoginTicket(fragment: string): string {
  const tickets = new URLSearchParams(fragment.replace(/^#/, '')).getAll('ticket')
  window.history.replaceState(
    window.history.state,
    '',
    window.location.pathname + window.location.search
  )
  return tickets.length === 1 && /^[A-Za-z0-9_-]{43}$/.test(tickets[0])
    ? tickets[0]
    : ''
}

export function consumeDesktopLoginTicket(): string | null {
  const ticket = desktopLoginTicket.value
  desktopLoginTicket.value = null
  return ticket
}
