# Vue Router Configuration

## Overview

This directory contains the Vue Router configuration for the Sub2API frontend application. The router implements a comprehensive navigation system with authentication guards, role-based access control, and lazy loading.

## Files

- **index.ts**: Main router configuration with route definitions and navigation guards
- **meta.d.ts**: TypeScript type definitions for route meta fields

## Route Structure

### Persistent Workspace Layout

`index.ts` separates `standaloneRoutes` from `workspaceRoutes`. A single lazy-loaded
`AppLayout` parent owns all workspace children, using absolute child paths to retain
existing URLs, names and aliases. Pages no longer wrap themselves in `AppLayout`.
Navigation replaces only the nested `RouterView`; the logo, sidebar, header and
background remain mounted. Page content is not cached with `KeepAlive`.

Layout ownership is independent of authentication: MCP OAuth consent is standalone
despite requiring login, while the public Airwallex payment route retains its workspace.
Ops fullscreen, Stripe's `method` popup and public Model Plaza use `hideAppChrome`
route metadata. The layout keeps the outlet's DOM ancestry stable during query changes,
so payment SDK state and Ops filters are not recreated. `appLayoutClass` applies the
Accounts-specific shell styles only while that route is active.

`__tests__/layout-lifecycle.spec.ts` exercises real route records and mounted pages,
checking component identities, DOM identity, page disposal and presentation exceptions.
Persistent overlay owners observe committed route changes to close transient menus
without rebuilding the shell. Cancelled navigation alone preserves their state.
The sidebar retries a failed access lookup on the next page navigation; successful
results stay cached by the existing composable. The layout and common component
navigation tests cover these behaviors separately from page lifecycle assertions.

### Desktop Sign-In

`/auth/desktop` captures and removes the ticket fragment before loading the callback.
The router publishes it only after that navigation succeeds; cancelled or failed
navigation discards only its own pending ticket and leaves the current session intact.
The mounted callback pauses the old session before exchanging a valid ticket and
aborts that exchange on unmount or replacement. A late response cannot install a session.
Missing or malformed tickets do not prevent normal persisted-session restoration
on the next route. The router lifecycle and API exchange tests cover these boundaries.

### Public Routes (No Authentication Required)

| Path        | Component    | Description            |
| ----------- | ------------ | ---------------------- |
| `/login`    | LoginView    | User login page        |
| `/register` | RegisterView | User registration page |

### User Routes (Authentication Required)

| Path         | Component     | Description                  |
| ------------ | ------------- | ---------------------------- |
| `/`          | -             | Redirects to `/home`         |
| `/dashboard` | DashboardView | User dashboard with stats    |
| `/keys`      | KeysView      | API key management           |
| `/usage`     | UsageView     | Usage records and statistics |
| `/redeem`    | RedeemView    | Redeem code interface        |
| `/profile`   | ProfileView   | User profile settings        |

### Admin Routes (Admin Role Required)

| Path               | Component          | Description                     |
| ------------------ | ------------------ | ------------------------------- |
| `/admin`           | -                  | Redirects to `/admin/dashboard` |
| `/admin/dashboard` | AdminDashboardView | Admin dashboard                 |
| `/admin/users`     | AdminUsersView     | User management                 |
| `/admin/groups`    | AdminGroupsView    | Group management                |
| `/admin/accounts`  | AdminAccountsView  | Account management              |
| `/admin/proxies`   | AdminProxiesView   | Proxy management                |
| `/admin/redeem`    | AdminRedeemView    | Redeem code management          |

### Special Routes

| Path              | Component    | Description    |
| ----------------- | ------------ | -------------- |
| `/:pathMatch(.*)` | NotFoundView | 404 error page |

## Navigation Guards

### Authentication Guard (beforeEach)

The router implements a comprehensive navigation guard that:

1. **Sets Page Title**: Updates document title based on route meta
2. **Checks Authentication**:
   - Public routes (`requiresAuth: false`) are accessible without login
   - Protected routes require authentication
   - Redirects to `/login` if not authenticated
3. **Prevents Double Login**:
   - Redirects authenticated users away from login/register pages
4. **Role-Based Access Control**:
   - Admin routes (`requiresAdmin: true`) require admin role
   - Non-admin users are redirected to `/dashboard`
5. **Preserves Intended Destination**:
   - Saves original URL in query parameter for post-login redirect

### Flow Diagram

```
User navigates to route
        ↓
Set page title from meta
        ↓
Is route public? ──Yes──→ Already authenticated? ──Yes──→ Redirect to /dashboard
        ↓ No                                        ↓ No
        ↓                                      Allow access
        ↓
Is user authenticated? ──No──→ Redirect to /login with redirect query
        ↓ Yes
        ↓
Requires admin role? ──Yes──→ Is user admin? ──No──→ Redirect to /dashboard
        ↓ No                                  ↓ Yes
        ↓                                     ↓
Allow access ←────────────────────────────────┘
```

## Route Meta Fields

Each route can define the following meta fields:

```typescript
interface RouteMeta {
  requiresAuth?: boolean // Default: true (requires authentication)
  requiresAdmin?: boolean // Default: false (admin access only)
  title?: string // Page title
  breadcrumbs?: Array<{
    // Breadcrumb navigation
    label: string
    to?: string
  }>
  icon?: string // Icon for navigation menu
  hideInMenu?: boolean // Hide from navigation menu
}
```

## Lazy Loading

All route components use dynamic imports for code splitting:

```typescript
component: () => import('@/views/user/DashboardView.vue')
```

Benefits:

- Reduced initial bundle size
- Faster initial page load
- Components loaded on-demand
- Automatic code splitting by Vite

## Authentication Store Integration

The router integrates with the Pinia auth store (`@/stores/auth`):

```typescript
const authStore = useAuthStore()

// Check authentication status
authStore.isAuthenticated

// Check admin role
authStore.isAdmin
```

## Usage Examples

### Programmatic Navigation

```typescript
import { useRouter } from 'vue-router'

const router = useRouter()

// Navigate to a route
router.push('/dashboard')

// Navigate with query parameters
router.push({
  path: '/usage',
  query: { filter: 'today' }
})

// Navigate to admin route (will be blocked if not admin)
router.push('/admin/users')
```

### Route Links

```vue
<template>
  <!-- Simple link -->
  <router-link to="/dashboard">Dashboard</router-link>

  <!-- Named route -->
  <router-link :to="{ name: 'Keys' }">API Keys</router-link>

  <!-- With query parameters -->
  <router-link :to="{ path: '/usage', query: { page: 1 } }"> Usage </router-link>
</template>
```

### Checking Current Route

```typescript
import { useRoute } from 'vue-router'

const route = useRoute()

// Check if on admin page
const isAdminPage = route.path.startsWith('/admin')

// Get route meta
const requiresAdmin = route.meta.requiresAdmin
```

## Scroll Behavior

The router implements automatic scroll management:

- **Browser Navigation**: Restores saved scroll position
- **New Routes**: Scrolls to top of page
- **Hash Links**: Scrolls to anchor (when implemented)

## Error Handling

The router includes error handling for navigation failures:

```typescript
router.onError((error) => {
  console.error('Router error:', error)
})
```

## Testing Routes

To test navigation guards and route access:

1. **Public Route Access**: Visit `/login` without authentication
2. **Protected Route**: Try accessing `/dashboard` without login (should redirect)
3. **Admin Access**: Login as regular user, try `/admin/users` (should redirect to dashboard)
4. **Admin Success**: Login as admin, access `/admin/users` (should succeed)
5. **404 Handling**: Visit non-existent route (should show 404 page)

## Development Tips

### Adding New Routes

1. Add route definition in `routes` array
2. Create corresponding view component
3. Set appropriate meta fields (`requiresAuth`, `requiresAdmin`)
4. Use lazy loading with `() => import()`
5. Update this README with route documentation

### Debugging Navigation

Enable Vue Router debug mode:

```typescript
// In browser console
window.__VUE_ROUTER__ = router

// Check current route
router.currentRoute.value
```

### Common Issues

**Issue**: 404 on page refresh

- **Cause**: Server not configured for SPA
- **Solution**: Configure server to serve `index.html` for all routes

**Issue**: Navigation guard runs twice

- **Cause**: Multiple `next()` calls
- **Solution**: Ensure only one `next()` call per code path

**Issue**: User data not loaded

- **Cause**: Auth store not initialized
- **Solution**: Call `authStore.checkAuth()` in App.vue or main.ts

## Security Considerations

1. **Client-Side Only**: Navigation guards are client-side; server must also validate
2. **Token Validation**: API should verify JWT token on every request
3. **Role Checking**: Backend must verify admin role, not just frontend
4. **XSS Protection**: Vue automatically escapes template content
5. **CSRF Protection**: Use CSRF tokens for state-changing operations

## Performance Optimization

1. **Lazy Loading**: All routes use dynamic imports
2. **Code Splitting**: Vite automatically splits route chunks
3. **Prefetching**: Consider adding route prefetch for common paths
4. **Route Caching**: Vue Router caches component instances

## Future Enhancements

- [ ] Add breadcrumb navigation system
- [ ] Implement route-based permissions beyond admin/user
- [ ] Add route transition animations
- [ ] Implement route prefetching for anticipated navigation
- [ ] Add navigation analytics tracking
