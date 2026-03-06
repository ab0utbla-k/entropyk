# entropyk

> "Out of chaos, find simplicity." — Bruce Lee

Chaos engineering operator for Kubernetes. Verify that services handle failure gracefully — not just in theory, but in practice.

- **Observability-first** — deep integration with PromQL-compatible metrics, not just "inject fault and hope"
- **Adaptive safeguards** — halts experiments on real deviations from baseline metrics
- **Recovery time tracking** — measures MTTR per (service, scenario) pair, flags regressions
- **Focused scope** — a few experiment types, easy to understand, audit, trust
- **Unprivileged by default** — uses K8s-native primitives, no privileged DaemonSet required
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
