<template>
  <AuthLayout>
    <section class="text-center" aria-labelledby="desktop-login-title">
      <h2 id="desktop-login-title" class="text-lg font-semibold text-gray-900 dark:text-white">
        {{ t(errorKey ? 'auth.desktopLogin.failedTitle' : 'auth.desktopLogin.title') }}
      </h2>
      <p v-if="errorKey" class="mt-4 text-sm text-gray-600 dark:text-gray-400" role="alert">
        {{ t(errorKey) }}
      </p>
      <div v-else role="status" aria-live="polite" aria-busy="true">
        <div
          class="mx-auto mt-6 h-8 w-8 animate-spin rounded-full border-2 border-primary-500 border-t-transparent"
          aria-hidden="true"
        ></div>
        <p class="mt-4 text-sm text-gray-600 dark:text-gray-400">
          {{ t('auth.desktopLogin.processing') }}
        </p>
      </div>
      <button
        v-if="errorKey"
        type="button"
        class="btn btn-primary mt-6 w-full"
        @click="router.replace('/login')"
      >
        {{ t('auth.backToLogin') }}
      </button>
    </section>
  </AuthLayout>
</template>

<script setup lang="ts">
import { onBeforeUnmount, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRouter } from 'vue-router'
import { AuthLayout } from '@/components/layout'
import { useAuthStore } from '@/stores/auth'
import { consumeDesktopLoginTicket, desktopLoginTicket } from '@/router/desktopLogin'
import { extractApiErrorCode } from '@/utils/apiError'

const { t } = useI18n()
const router = useRouter()
const authStore = useAuthStore()
const errorKey = ref(desktopLoginTicket.value === null ? 'auth.desktopLogin.invalid' : '')
let exchangeController: AbortController | null = null
onBeforeUnmount(() => exchangeController?.abort())

watch(desktopLoginTicket, async (pendingTicket) => {
  if (pendingTicket === null) return
  exchangeController?.abort()
  const controller = new AbortController()
  exchangeController = controller
  const ticket = consumeDesktopLoginTicket()
  errorKey.value = ''
  if (!ticket) {
    errorKey.value = 'auth.desktopLogin.invalid'
    return
  }

  authStore.suspendSession()
  try {
    await authStore.loginFromDesktop(ticket, controller.signal)
    controller.signal.throwIfAborted()
    await router.replace('/dashboard')
  } catch (error) {
    if (controller.signal.aborted) return
    const status = (error as { status?: number })?.status
    switch (extractApiErrorCode(error)) {
      case 'INVALID_BROWSER_LOGIN_TICKET':
        errorKey.value = 'auth.desktopLogin.invalid'
        break
      case 'BACKEND_MODE_ADMIN_ONLY':
        errorKey.value = 'auth.errors.BACKEND_MODE_ADMIN_ONLY'
        break
      default:
        errorKey.value = status === 0 ? 'auth.desktopLogin.network' : 'auth.desktopLogin.unavailable'
    }
  }
}, { immediate: true, flush: 'sync' })
</script>
