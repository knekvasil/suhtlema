| model                          |       size |     params | backend    | threads |            test |                  t/s |
| ------------------------------ | ---------: | ---------: | ---------- | ------: | --------------: | -------------------: |
| qwen3 4B Q4_K - Medium         |   2.32 GiB |     4.02 B | MTL,BLAS   |       4 |           pp512 |        188.78 ± 1.45 |
| qwen3 4B Q4_K - Medium         |   2.32 GiB |     4.02 B | MTL,BLAS   |       4 |          pp2048 |        177.11 ± 0.93 |
| qwen3 4B Q4_K - Medium         |   2.32 GiB |     4.02 B | MTL,BLAS   |       4 |           tg128 |         17.19 ± 1.75 |
| llama 3B Q4_K - Medium         |   1.87 GiB |     3.21 B | MTL,BLAS   |       4 |           pp512 |        253.81 ± 1.33 |
| llama 3B Q4_K - Medium         |   1.87 GiB |     3.21 B | MTL,BLAS   |       4 |          pp2048 |        238.00 ± 1.49 |
| llama 3B Q4_K - Medium         |   1.87 GiB |     3.21 B | MTL,BLAS   |       4 |           tg128 |         21.53 ± 0.21 |

build: c250304 (1)
