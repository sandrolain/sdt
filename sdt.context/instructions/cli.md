# CLI Usage & Examples

Curated command catalog: the main commands grouped by area, with conventions
and practical examples. Hand-maintained; for the complete, always-current
reference use the generated docs:

```
sdt manifest --format json           # full command tree
sdt schema --command "<command>"     # JSON Schema for one command
sdt context docs                     # per-command docs in sdt.context/docs/
sdt <command> --help                 # usage for a single command
```

## Conventions

- Input: stdin | `--input "<string>"` | `--file <path>` | `--inb64 <base64>`
- Output: `--format text|json|yaml` (default text); errors to stderr + non-zero exit
- `--quiet` suppresses informational output; `--no-color` disables ANSI
- Project identity: `--project` / `--group` flags or `.sdt.yaml` (found walking up)

## Agent tooling

- `sdt memory set|get|search|list|delete|export|import` — persistent memory (see memory.md)
- `sdt context new|path|list|task|docs` — plan/worklog/notes, task list, generated docs
- `sdt template --tmpl` — render Go templates from JSON/YAML data
- `sdt extract --type urls|emails|ips|json-blocks|code-blocks|dates`
- `sdt env parse|get|set|merge` — .env handling
- `sdt diff --a A --b B --diff-format unified|json-patch`

## Encoding & hashing

- `sdt b64|b32|b64url|hex|url|html [dec]` — encode/decode
- `sdt sha256|sha1|sha384|sha512|md5` — hashes
- `sdt bcrypt|bcrypt verify`, `sdt hmac --key`, `sdt keypair`, `sdt sign|verify`, `sdt cert inspect|expiry`

## Data & conversion

- `sdt conv --in json|yaml|toml|csv|msgpack --out ...`
- `sdt json pretty|minify|valid`

## IDs, time, strings

- `sdt uid v4|nano|ks`, `sdt time unix|iso|http`
- `sdt string uppercase|lowercase|titlecase|count|escape|unescape|replacespace`
- `sdt regexp|regexp replace --expression`

## Network

- `sdt http`, `sdt ipinfo`, `sdt nslookup`, `sdt dns --host --type A|AAAA|MX|TXT|CNAME|NS|PTR`, `sdt port`

## Other

- `sdt gzip|gunzip`, `sdt password`, `sdt qrcode|qrcode read`, `sdt totp uri|code|verify`, `sdt vman`, `sdt config get|set`, `sdt version`

## Examples

```
echo "hello" | sdt b64                                  # encode
echo "password" | sdt sha256                            # hash
echo "payload" | sdt hmac --key "secret"                # hmac
echo '{"a":1}' | sdt conv --in json --out yaml          # convert
echo '{"user":"Alice"}' | sdt template --tmpl "Hi {{.user}}"
cat llm_response.txt | sdt extract --type urls
cat config.yaml | sdt conv --in yaml --out json
echo "password" | sdt sha256 | sdt hex                  # pipeline
sdt diff --a old.json --b new.json --diff-format json-patch
sdt memory set "arch:database" "PostgreSQL" --project myapp --tags "database,architecture"
sdt memory search "project architecture" --project myapp --format json
sdt totp code --secret BASE32SECRET
sdt dns --host example.com --type A --format json
```
