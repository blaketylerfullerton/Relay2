## Relay 2

### Overview
Relay is a WAN native runtime that turns geographically distributed heterogenious GPUs into a simple programmable AI compute fabric

**Relay is the way to distribute an entire AI system**

Relay schedules the entire DAG

One way to think about it

Docker: 
    - Package one application

Kubernetes:
    - Coordinate Thousands of containers

Ollama:
    - Run one model

Petals:
    - Split one model

Relay:
    - Coordinate an entire AI Infrustructure

Why this is different. 
    The hardest part is not about splitting tranformer layers.
    The hardest part is building a scheduler that can asnwer:
        - Which node should run this
        - Should i replicate KV cahce
        - is WAN latency worth paying 
        - Should speculative deocding happen locally
        - Is bandwhich saturated


Phase 0: Building the scheduler not the inference. We are NOt trying to invednt a new distrubuteed transformer architecture. We are building a runtime that orchestrates the existing inference servers.

```
relay-agent install

Machine A
- RTX 4090
- vLLM
- 24GB VRAM

Machine B
- Mac Studio
- llama.cpp
- 64GB Unified Memory

Machine C
- H100
- TensorRT-LLM
```

Each agent periodically reports:
Hardware
Available RAM
Current Utilization
RTT to peers
Bandwitdh estimates
Running models
Health status

At tjhis point, relay is basically a GPU inventory system

Phase 1: "SSH for AI"
We just type `relay nodes`
Ouput:
```
NAME            GPU     FREE
office-west     H100    716GB
home-pc         4090    18GB
```

Then
```
relay exec home-pc ollama run llama3
```
or
```
relay run llama-70b
```

Phase 2: Automatic placement
simply say:
```
relay run deepseek-r1
```

Scheduler decides:
- Enough VRAM?
- Lowest latency?
- already cached?
- GPU current idle
- Cheapest nodes?

Starting to look like kubernetes clustering

Phase 3: DAG Execution
```
PDF Upload

    │

Embedding Model

    │

Retriever

    │

Reranker

    │

LLM

    │

Voice Synthesizer
```