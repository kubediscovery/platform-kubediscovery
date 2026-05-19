# KUBEDISCOVERY AGENT (KD-AGENT)

## Overview

O `kd-agent` é o cliente gRPC do ecossistema Kubediscovery. Ele roda dentro (ou próximo) de cada cluster/nó monitorado e mantém uma conexão bidirecional persistente com o `kd-gateway` via **mTLS**, enviando heartbeats periódicos e respondendo a probes iniciados pelo servidor.

---

## Arquitetura

```
kd-agent (cliente gRPC)
    │
    │  mTLS
    │  - apresenta: client.crt / client.key  (emitido pelo kdctl certificate)
    │  - valida:    ca.crt do servidor        (emitido pelo kdctl init)
    ▼
kd-gateway (servidor gRPC : addr via GRPC_ADDR)
```

### Goroutines internas de `runHealthStream`

```
runHealthStream()
    │
    ├─ goroutine: Sender
    │   └─ lê de sendCh (buffer 64) → stream.Send(msg)
    │       → única goroutine que escreve no stream (evita concurrent Send)
    │
    ├─ goroutine: Ticker (heartbeat)
    │   ├─ envia request imediatamente ao conectar
    │   └─ repete a cada 10s via time.Ticker
    │       → envia para sendCh: newRequest(callerID, {"kind": "agent->server"})
    │
    ├─ goroutine: Receiver
    │   ├─ msg.GetRequest()  → server probe → responde via sendCh
    │   │       newResponse(req.GetId(), callerID, {"kind": "server->agent"})
    │   └─ msg.GetResponse() → loga: request_id, responder_id, checks, next_check_in
    │
    └─ main goroutine: bloqueia em select { ctx.Done | errCh }
```

---

## Identificação do Agente via Metadata

Esta é a parte **crítica** do agente: o gateway precisa saber **qual agente está conectado**. A identidade é composta por três camadas.

### Camadas de Identidade

```
Cada mensagem enviada pelo agente ao gateway
        │
        ├─ 1. caller_id  (campo HealthCheckRequest.caller_id)
        │       → definido pela variável AGENT_ID (padrão: "kd-agent")
        │       → enviado em TODA mensagem (request e response)
        │       → é a chave com que o gateway indexa o agente em memória:
        │             clientConnected[caller_id] = &HealthClient{...}
        │
        ├─ 2. metadata   (google.protobuf.Struct — mapa livre chave/valor)
        │       → em requests  (agent → gateway):  {"kind": "agent->server"}
        │       → em responses (agent → gateway):  {"kind": "server->agent"}
        │       → campo extensível: adicionar hostname, versão, labels, cluster, etc.
        │
        └─ 3. Certificado TLS do cliente (mTLS — identidade CONFIÁVEL)
                → apresentado automaticamente pelo tls.Config.Certificates
                → lido pelo gateway como PeerCertificates[0]
                → o Common Name / SAN do certificado é o cert_name no gateway
                → configurado via GRPC_CLIENT_CERT_FILE / GRPC_CLIENT_KEY_FILE
```

### Campo `metadata` — Estrutura Atual e Como Estender

O campo `metadata` é um `google.protobuf.Struct` (JSON livre em protobuf). Hoje o agente envia apenas `kind`:

```go
// request (heartbeat agent → gateway)
newRequest(callerID, map[string]any{"kind": "agent->server"})

// response (agente respondendo probe do servidor)
newResponse(req.GetId(), callerID, map[string]any{"kind": "server->agent"})
```

Para identificar o agente com mais precisão, basta enriquecer o mapa:

```go
map[string]any{
    "kind":      "agent->server",
    "hostname":  os.Hostname(),
    "version":   "v1.2.0",
    "cluster":   "prod-us-east-1",
    "namespace": "monitoring",
}
```

> O gateway já loga `metadata` completo em cada mensagem recebida:
> `log.Printf("... metadata=%v", req.GetMetadata())`

### Como o `caller_id` chega ao gateway

```go
// Em newRequest():
Request: &health.HealthCheckRequest{
    Id:       fmt.Sprintf("%d", time.Now().UnixNano()),  // ID único por mensagem
    CallerId: callerID,                                   // ← AGENT_ID env var
    SentAt:   timestamppb.Now(),
    Timeout:  durationpb.New(5 * time.Second),
    Metadata: md,                                         // ← structpb.Struct
}
```

---

## Fluxo Completo de Conexão

```
main()
    │
    ├─ lê configuração via env vars
    ├─ newClientCreds(...) → monta tls.Config com CA + client cert
    ├─ signal.NotifyContext(SIGINT, SIGTERM) → ctx com cancelamento
    │
    └─ retry loop (maxRetries=5, weight=3, baseDelay=1s)
            │
            ├─ attempt 0 → delay: -
            ├─ attempt 1 → delay: 1s
            ├─ attempt 2 → delay: 3s
            ├─ attempt 3 → delay: 9s
            ├─ attempt 4 → delay: 27s
            └─ attempt 5 → delay: 81s  → log.Fatalf (desiste)
                    │
                    └─ runHealthStream(ctx, creds, addr, callerID, onConnect)
                            │
                            ├─ grpc.NewClient(addr, WithTransportCredentials)
                            ├─ client.HealthStream(streamCtx)
                            ├─ onConnect() → reseta attempt para 0
                            │
                            ├─ [Sender goroutine]    ← lê sendCh → stream.Send
                            ├─ [Ticker goroutine]    ← heartbeat a cada 10s
                            └─ [Receiver goroutine]  ← trata requests e responses
```

---

## Fluxo: `HealthStream` (bidirecional)

```
Agente → Gateway (heartbeat a cada 10s)
─────────────────────────────────────────────────────
HealthStreamMessage {
  request: HealthCheckRequest {
    id:        "<unix_nano>"
    caller_id: "<AGENT_ID>"
    sent_at:   <now>
    timeout:   5s
    metadata:  { "kind": "agent->server" }
  }
}

Gateway → Agente (probe do servidor)
─────────────────────────────────────────────────────
HealthStreamMessage {
  request: HealthCheckRequest { id: "<uuid>", caller_id: "kd-gateway", ... }
}

Agente → Gateway (resposta ao probe)
─────────────────────────────────────────────────────
HealthStreamMessage {
  response: HealthCheckResponse {
    request_id:   "<id do request recebido>"
    responder_id: "<AGENT_ID>"
    responded_at: <now>
    next_check_in: 10s
    checks: [{ name: "overall", code: UP, message: "ok" }]
    metadata: { "kind": "server->agent" }
  }
}

Gateway → Agente (confirmação)
─────────────────────────────────────────────────────
HealthStreamMessage {
  response: HealthCheckResponse {
    request_id:    "<id do heartbeat>"
    responder_id:  "kd-gateway"
    next_check_in: 10s
    checks: [{ name: "overall", code: UP, message: "received" }]
  }
}
→ agente loga: request_id, responder_id, checks count, next_check_in
```

---

## Fluxo: `HealthClientStream` (consulta de clientes)

Função `healthClientsStream` — atualmente **comentada** no `main`, disponível para ativação:

```
Agente → Gateway
─────────────────────────────────────────────────────
HealthClientRequest {
  id:        "<uuid>"
  caller_id: "<AGENT_ID>"
}

Gateway → Agente (server-side stream)
─────────────────────────────────────────────────────
HealthClientResponse {
  request_id:   "<uuid>"
  responder_id: "kd-gateway"
  clients: [
    { id: "kd-agent-1", name: "...", code: UP, peerIP: "10.42.1.23" },
    { id: "kd-agent-2", name: "...", code: UP, peerIP: "10.42.1.24" },
    ...
  ]
}
→ agente imprime: ID | Nome | Status | IP de cada cliente conectado
```

---

## Configuração mTLS (`newClientCreds`)

```go
tls.Config{
    RootCAs:            pool,             // CA do servidor (valida kd-gateway)
    Certificates:       []clientCert,     // cert+key do agente (apresentado ao gateway)
    ServerName:         serverName,       // hostname esperado no cert do gateway
    InsecureSkipVerify: false,            // configurável via GRPC_INSECURE_SKIP_VERIFY
    MinVersion:         tls.VersionTLS12,
}
```

Resolução de `serverName`:
1. `GRPC_SERVER_NAME` env var (se definido)
2. Hostname extraído de `GRPC_ADDR` (se não for IP)
3. Vazio → Go usa o hostname do endereço na verificação TLS

---

## Variáveis de Ambiente

| Variável | Padrão | Descrição |
|---|---|---|
| `AGENT_ID` | `kd-agent` | **Identificador lógico do agente** — vira `caller_id` em todas as mensagens |
| `GRPC_ADDR` | `localhost:50051` | Endereço do `kd-gateway` |
| `GRPC_CA_FILE` | `~/.kubediscovery/certs/staging/ca.crt` | CA que assina o certificado do servidor |
| `GRPC_CLIENT_CERT_FILE` | `~/.kubediscovery/certs/staging/srv004.crt` | Certificado mTLS do agente |
| `GRPC_CLIENT_KEY_FILE` | `~/.kubediscovery/certs/staging/srv004.key` | Chave privada do agente |
| `GRPC_SERVER_NAME` | `""` (usa hostname de GRPC_ADDR) | Override do ServerName TLS |
| `GRPC_INSECURE_SKIP_VERIFY` | `false` | Desabilita verificação do cert do servidor (dev only) |

> `AGENT_ID` é a variável mais importante para identificação: deve ser **única por instância** de agente (ex: nome do nó, nome do cluster, ou UUID).

---

## Bibliotecas Utilizadas

| Biblioteca | Versão | Propósito |
|---|---|---|
| `google.golang.org/grpc` | v1.80.0 | Framework gRPC — cliente, streaming, credentials |
| `google.golang.org/protobuf` | v1.36.11 | Runtime protobuf — `timestamppb`, `durationpb`, `structpb` |
| `github.com/google/uuid` | — | Geração de UUID para `HealthClientRequest.id` |
| `github.com/kubediscovery/kd-libs` | local (`../../libs`) | Definições proto compiladas (`health.pb.go`, `health_grpc.pb.go`) |

---

## Arquivos do Módulo

| Arquivo | Responsabilidade |
|---|---|
| `cmd/grpc/main.go` | Entry point completo: configuração mTLS, retry loop, `runHealthStream`, `healthClientsStream`, `newRequest`, `newResponse`, `newClientCreds` |
