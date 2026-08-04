# Planning and Work Logs

Keep planning, work logs and annotations under the `sdt.context/` directory:

```
sdt.context/plan/<YYYY-MM-DD>-<slug>.md              # plan before starting work
sdt.context/worklog/<YYYYMMDD-HHMMSS>-<slug>.md      # ordered log of completed work
sdt.context/notes/<YYYYMMDD-HHMMSS>-<slug>.md        # free-form annotations
```

Temporary and scratch files live in `sdt.context/tmp/` — never outside the
project (no `/tmp`, no other absolute paths).

Date/time prefixes keep files sortable and provide a durable history.

Every work file starts with YAML frontmatter carrying metadata:

```yaml
---
kind: worklog      # plan | worklog | notes
created_at: 2026-08-02T10:00:00Z
context: what triggered this entry
project: sdt_44f24890
---
```