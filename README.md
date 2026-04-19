# temper

> "Out of chaos, find simplicity." — Bruce Lee

Chaos engineering operator for Kubernetes. Verify that services handle failure gracefully — not just in theory, but in practice.

- **Adaptive safeguards** — seasonal baselines (mean ± n·σ by hour-of-day × weekday); halts on real deviation from the learned pattern, not a static threshold
- **Capability-scoped agent** — no `privileged: true`; opt-in `NET_ADMIN`/`SYS_TIME` only for scenarios that need kernel access
- **Recovery-time regression detection** — MTTR tracked per (service, scenario); flags when recovery exceeds the learned baseline
- **Focused scope** — a handful of scenarios, easy to audit and trust
- **GitOps-friendly** — just CRDs and a controller, no UI, no hub

## Status

Work in progress. Not ready for production use.

## License

Copyright 2026.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
