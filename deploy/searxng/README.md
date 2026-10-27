# SearXNG for the research lane

A local SearXNG instance, so `sdt research search` has a provider that costs nothing per
query and never sends the query to a vendor.

## Run it

```bash
cd deploy/searxng
cp searxng/settings.yml.example searxng/settings.yml   # first time only
# replace server.secret_key with: openssl rand -hex 32
docker compose up -d
```

The instance answers on `http://127.0.0.1:8888/`; a JSON search is

```bash
curl 'http://127.0.0.1:8888/search?q=searxng&format=json' | head -c 200
```

`format=json` is enabled in `searxng/settings.yml`; upstream SearXNG turns it off by
default, and without it the provider refuses every request.

## Point the project at it

```yaml
research:
  - provider: tavily
  - provider: firecrawl
  - provider: exa
  - provider: searxng
    endpoint: "http://127.0.0.1:8888"
```

SearXNG has no key: its availability is the declared endpoint. An entry without one is
skipped by the research walk, with the reason recorded on the run.
