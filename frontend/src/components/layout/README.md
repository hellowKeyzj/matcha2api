# Layout Components

Vue 3 layout components for the Sub2API frontend, built with Composition API, TypeScript, and TailwindCSS.

## Components

### 1. AppLayout.vue

Persistent router parent with sidebar, header and workspace background.

**Route ownership:**

```typescript
{
  path: '/',
  component: () => import('@/components/layout/AppLayout.vue'),
  children: [
    { path: '/dashboard', component: () => import('@/views/user/DashboardView.vue') },
    // Other workspace pages share this same parent.
  ]
}
```

Pages render only their content, without importing or wrapping `AppLayout`.
Authentication, setup, MCP consent and other standalone pages stay outside this parent.

**Features:**

- Responsive sidebar (collapsible)
- Fixed header at top
- Main content area with a nested `RouterView`
- Automatically adjusts margin based on sidebar state
- Preserves sidebar, logo, header and particles during workspace navigation
- `hideAppChrome` route meta hides workspace chrome for Ops fullscreen, Stripe popup
  and public Model Plaza without remounting the page when its query changes
- `appLayoutClass` route meta scopes page-specific shell styling (Accounts)

**Lifecycle ownership:**

- The router owns the shell; leaving a workspace page still disposes its content
- Each persistent menu/dialog owner closes its transient overlay on committed navigation
- Click-outside and explicit close actions still work; a cancelled navigation alone does not close overlays
- Sidebar access lookup retries after a failed request on the next page navigation,
  using the existing composable's cache and in-flight request deduplication

---

### 2. AppSidebar.vue

Navigation sidebar with user and admin sections.

**Features:**

- Logo/brand at top
- User navigation links:
  - Dashboard
  - API Keys
  - Usage
  - Redeem
  - Profile
- Admin navigation links (shown only if user is admin):
  - Admin Dashboard
  - Users
  - Groups
  - Accounts
  - Proxies
  - Redeem Codes
- Collapsible sidebar with toggle button
- Active route highlighting
- Icons using HTML entities
- Responsive (mobile-friendly)

**Used automatically by AppLayout** - no need to import separately.

---

### 3. AppHeader.vue

Top header with user info and actions.

**Features:**

- Mobile menu toggle button
- Page title from route metadata
- User balance display (desktop only)
- User dropdown menu with:
  - Profile link
  - Logout button
- User avatar with initials
- Click-outside handling for dropdown
- Responsive design

**Page titles come from route meta:**

```typescript
meta: { title: 'Custom Page Title', titleKey: 'customPage.title' }
```

**Used automatically by AppLayout** - no need to import separately.
The header also owns the onboarding controls, which are not mounted in standalone presentation.

---

### 4. AuthLayout.vue

Simple centered layout for authentication pages (login/register).

**Usage:**

```vue
<template>
  <AuthLayout>
    <!-- Login/Register form content -->
    <h2 class="mb-6 text-2xl font-bold">Login</h2>

    <form @submit.prevent="handleLogin">
      <!-- Form fields -->
    </form>

    <!-- Optional footer slot -->
    <template #footer>
      <p>
        Don't have an account?
        <router-link to="/register" class="text-indigo-600 hover:underline"> Sign up </router-link>
      </p>
    </template>
  </AuthLayout>
</template>

<script setup lang="ts">
import { AuthLayout } from '@/components/layout'

function handleLogin() {
  // Login logic
}
</script>
```

**Features:**

- Centered card container
- Gradient background
- Logo/brand at top
- Main content slot
- Optional footer slot for links
- Fully responsive

---

## Route Configuration

To set page titles in the header, add meta to your routes:

```typescript
// router/index.ts
const routes = [
  {
    path: '/dashboard',
    component: DashboardView,
    meta: { title: 'Dashboard' }
  },
  {
    path: '/api-keys',
    component: ApiKeysView,
    meta: { title: 'API Keys' }
  }
  // ...
]
```

---

## Store Dependencies

These components use the following Pinia stores:

- **useAuthStore**: For user authentication state, role checking, and logout
- **useAppStore**: For sidebar state management and toast notifications

Make sure these stores are properly initialized in your app.

---

## Styling

All components use TailwindCSS utility classes. Make sure your `tailwind.config.js` includes the component paths:

```js
module.exports = {
  content: ['./index.html', './src/**/*.{vue,js,ts,jsx,tsx}']
  // ...
}
```

---

## Icons

Components use HTML entity icons for simplicity:

- &#128200; Chart (Dashboard)
- &#128273; Key (API Keys)
- &#128202; Bar Chart (Usage)
- &#127873; Gift (Redeem)
- &#128100; User (Profile)
- &#128268; Admin
- &#128101; Users
- &#128193; Folder (Groups)
- &#127760; Globe (Accounts)
- &#128260; Network (Proxies)
- &#127991; Ticket (Redeem Codes)

You can replace these with your preferred icon library (e.g., Heroicons, Font Awesome) if needed.

---

## Mobile Responsiveness

All components are fully responsive:

- **AppSidebar**: Fixed positioning on desktop, hidden by default on mobile
- **AppHeader**: Shows mobile menu toggle on small screens, hides balance display
- **AuthLayout**: Adapts padding and card size for mobile devices

The sidebar uses Tailwind's responsive breakpoints (md:) to adjust behavior.
