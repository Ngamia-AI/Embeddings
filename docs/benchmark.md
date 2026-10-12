# Phase 0 benchmark (template)

Fill this in on the real VPS before choosing defaults. The SRS §11 Phase 0 gate
requires the RAM, latency and parity tables here, and the precision / slot /
budget decisions in `context/decisions.md`.

## Host

- `lscpu` (flags relevant to BF16: `avx512_bf16`, `amx_bf16`, ...):
- `free -m` at idle:
- Engine peak RSS (BF16 pair):
- Engine peak RSS (Q8_0 pair):

## Latency (p50 / p95, ms)

| case | concurrency | BF16 | Q8_0 |
| --- | --- | --- | --- |
| text, 1 input ~50 tok | 1 | | |
| text, 8 inputs ~200 tok | 1 | | |
| text, 32 inputs ~512 tok | 1 | | |
| text, 32 inputs ~512 tok | 2 | | |
| one 1 MP photo | 1 | | |
| audio 10 / 30 / 60 / 120 s | 1 | | |

## Parity vs `sentence-transformers` fp32 (cosine)

Reference: embed the same 50 texts, 20 images, 20 audio clips in fp32.

| modality | floor | BF16 | Q8_0 |
| --- | --- | --- | --- |
| text | >= 0.99 | | |
| image | >= 0.95 | | |
| audio | >= 0.95 | | |

## Quality sanity

- Swahili / English text pairs (nearest neighbor correct?):
- Image closer to its caption than a wrong caption:
- Swahili audio closer to its transcript than an unrelated sentence:

## Spikes (recorded, not built)

- Mixed text + image in one item (FR-5): supported? notes:
- Video as 1 fps frames vs the reference: notes:

## Decision

- Precision: BF16 / Q8_0 (why, per the decision rule):
- Slot count (`-np`) and item token budget:
- `MEDIA_CONCURRENCY`:
- Per-app default limits derived from these numbers:
