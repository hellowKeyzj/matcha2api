import type { SubscriptionType } from '@/types'

export function supportsSubscriptionType(type?: SubscriptionType | string | null): boolean {
  return type === 'subscription' || type === 'subscription_balance'
}

export function isSubscriptionBalanceType(type?: SubscriptionType | string | null): boolean {
  return type === 'subscription_balance'
}
