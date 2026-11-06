# Runtime subpath deployment

The same image supports root and subpath deployments. No frontend rebuild or
database migration is needed when changing the mount path.

Set `APP_BASE_PATH=/sub2api` in the Compose `.env` file and recreate the service:

```sh
docker compose up -d sub2api
```

An unset value or `/` keeps the default root deployment. A configured container
serves one public mount, not two interchangeable public URLs. `/health` remains
available at the root for existing container health checks.

Preserve the prefix at the reverse proxy. Inside the existing Caddy site's
`route` block, before its fallback handler:

```caddyfile
handle /sub2api {
    redir /sub2api/ 308
}
handle /sub2api/* {
    reverse_proxy 47.251.94.80:8080
}
```

Use `handle`, NOT `handle_path`: the application strips the prefix itself.
The frontend, API and gateway then use `/sub2api/`, `/sub2api/api/v1/` and
`/sub2api/v1/` respectively. The root deployment uses the original paths.

Set the application's public/frontend URL to
`https://www.supercnm.top/sub2api`. Update explicitly configured OAuth callback
URLs, payment webhook/return URLs and external client base URLs to include the
prefix, including their registrations with third-party providers. Absolute
external URLs are not rewritten automatically. Test these integrations in the
provider sandbox before enabling live payments or social sign-in.
