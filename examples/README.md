# Example catalogs

These small FBC fixtures use neutral catalog and package names so the viewer
can be demonstrated without depending on a vendor catalog. They are derived
from actual FBC documents, but intentionally contain only the records needed
for focused examples.

The examples use this local-cache layout:

```text
catalog-tour/
└── aurora-catalog/
    ├── 2024.1/aurora-catalog-v2024.1.yaml
    └── 2024.2/aurora-catalog-v2024.2.yaml
```

Run the complete tour with:

```sh
go run . serve \
  --catalog-dir examples/catalog-tour \
  --catalog aurora-catalog \
  --releases 2024.1,2024.2 \
  --skip-range-edges
```

## Scenarios

| Scenario | Viewer focus | Suggested command |
| --- | --- | --- |
| Cross-revision continuity | Same package across revision planes | `--package lumen-cache` |
| Upgrade paths | `replaces`, `skips`, and `skipRange` links | `--package lumen-cache --skip-range-edges` |
| Dependencies | Package dependency links | `--package quartz-ingress` |
| Node states | Channel heads, deprecated, and phantom nodes | `--package orbit-metrics` |
| Filtering | Search and dependency visibility controls | Start without `--package` |

The generated README graphics live in `../docs/images/` and are intentionally
cropped around the relevant graph region rather than showing a dense catalog.
