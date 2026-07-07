# Linearizability Model & Correctness Properties

Linearizability guarantees that all operations appear to execute atomically at some point in time between their invocation and response.

## Core Properties
1. **Real-time Order**: If operation B starts after operation A completes, B must see the effects of A.
2. **Total Order**: All operations can be sequenced in a single, consistent execution history.

## Verification
In testing, we use linearizability checkers like **Porcupine** to validate client operation histories under network partitions and failures.
