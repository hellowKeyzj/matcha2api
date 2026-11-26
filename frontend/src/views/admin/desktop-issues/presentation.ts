import type { DesktopIssue, DesktopIssueSeverity, DesktopIssueStatus } from '@/api/admin/issues'

export const issueStatuses: DesktopIssueStatus[] = ['open', 'in_progress', 'resolved', 'closed']
export const issueSeverities: DesktopIssueSeverity[] = ['blocker', 'major', 'minor']

export const statusStyle = {
  open: {
    dot: 'bg-emerald-500',
    badge: 'border-emerald-500/30 bg-emerald-500/10 text-emerald-700 dark:text-emerald-400'
  },
  in_progress: {
    dot: 'bg-amber-500',
    badge: 'border-amber-500/30 bg-amber-500/10 text-amber-700 dark:text-amber-400'
  },
  resolved: {
    dot: 'bg-violet-500',
    badge: 'border-violet-500/30 bg-violet-500/10 text-violet-700 dark:text-violet-400'
  },
  closed: {
    dot: 'bg-gray-400',
    badge: 'border-gray-200 bg-gray-100 text-gray-600 dark:border-dark-600 dark:bg-dark-800 dark:text-dark-400'
  }
} satisfies Record<DesktopIssueStatus, { dot: string; badge: string }>

export const severityStyle = {
  blocker: 'bg-rose-500/10 text-rose-700 dark:text-rose-400',
  major: 'bg-orange-500/10 text-orange-700 dark:text-orange-400',
  minor: 'bg-gray-100 text-gray-600 dark:bg-dark-800 dark:text-dark-400'
} satisfies Record<DesktopIssueSeverity, string>

export function reporterDisplay(issue: DesktopIssue): string {
  return issue.reporter_email || issue.reporter_username || (issue.reporter_user_id ? `user#${issue.reporter_user_id}` : '-')
}

export function initialChar(name: string): string {
  return name.trim().slice(0, 1).toUpperCase()
}

export function shortOs(os: string): string {
  return os.split(/\s+/).slice(0, 2).join(' ')
}
