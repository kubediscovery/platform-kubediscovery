# KUBEDISCOVERY GATEWAY SERVER (KD-GATEWAY)

## Overview

O `kd-gateway` é o servidor gRPC central do ecossistema Kubediscovery. Ele recebe conexões dos agentes (`kd-agent`) via **mTLS** e mantém um registro em memória de todos os clientes ativos, identificando cada um por meio de uma combinação de campos do protocolo e da identidade do certificado TLS.

---

## Arquitetura

```
kd-agent (cliente gRPC)
    │
    │  mTLS (certificado emitido pelo CA do kdctl init)
    ▼
kd-gateway (servidor gRPC : 0.0.0.0:50051)
    │
    ├─ Interceptors (sempre ativos)
    │   ├─ UnaryInterceptors   → log de método + request
    │   └─ StreamInterceptors  → log de início/fim + peer + duração
    │
    ├─ Debug Interceptors (GRPC_DEBUG=1)
    │   ├─ unaryDebugInterceptor
    │   └─ streamDebugInterceptor
    │
    └─ HealthService (serviço registrado)
        ├─ HealthStream          → stream bidirecional (heartbeat)
        └─ HealthClientStream    → server-side stream (lista clientes conectados)
```

---

## Serviço gRPC: `HealthService`

Definido em `libs/core/v1/proto/health.proto`.

```protobuf
service HealthService {
  // Stream bidirecional: cliente e servidor trocam HealthStreamMessage
  rpc HealthStream(stream HealthStreamMessage) returns (stream HealthStreamMessage);

  // Server-side stream: retorna todos os clientes conectados no momento
  rpc HealthClientStream(HealthClientRequest) returns (stream HealthClientResponse);
}
```

### Mensagens principais

| Mensagem | Direção | Conteúdo |
|---|---|---|
| `HealthCheckRequest` | Client → Server | `id`, `caller_id`, `sent_at`, `timeout`, **`metadata`** |
| `HealthCheckResponse` | Server → Client | `request_id`, `responder_id`, `responded_at`, `next_check_in`, `checks[]` |
| `HealthStreamMessage` | Bidirecional | Envelope com `oneof payload { request \| response }` |
| `HealthClientRequest` | Client → Server | `id`, `caller_id` |
| `HealthClientResponse` | Server → Client | `request_id`, `responder_id`, `clients[]` |
| `HealthClient` | — | `id`, `name`, `code`, `peerIP` |

---

## Identificação do Cliente via Metadata

Esta é a parte **crítica** do gateway: para saber qual cliente está conectado, o servidor combina três fontes de identidade em ordem de confiabilidade.

### Fontes de Identidade

```
HealthCheckRequest recebido
        │
        ├─ 1. caller_id  (campo obrigatório, auto-reportado pelo cliente)
        │       → chave primária do registro em memória
        │       → usado como ID em lastByCaller[] e clientConnected[]
        │
        ├─ 2. metadata   (google.protobuf.Struct — mapa livre de chave/valor)
        │       → dados extras que o cliente envia (versão, hostname, labels, etc.)
        │       → logado integralmente: log.Printf("... metadata=%v", req.GetMetadata())
        │
        └─ 3. Certificado TLS do cliente (mTLS — identidade CONFIÁVEL)
                → extraído de peer.AuthInfo.(credentials.TLSInfo)
                → PeerCertificates[0] (leaf certificate)
                → resolução de nome em ordem de prioridade:
                    1º  SAN URI  (leaf.URIs[0].String())
                    2º  SAN DNS  (leaf.DNSNames[0])
                    3º  CN       (leaf.Subject.CommonName)   ← fallback
                → registrado como cert_name no log
```

### Campo `metadata` — Detalhes

O campo `metadata` em `HealthCheckRequest` é um `google.protobuf.Struct`, ou seja, um JSON livre serializado em protobuf. O cliente pode enviar qualquer conjunto de chaves/valores:

```json
{
  "hostname": "node-prod-42",
  "version": "v1.3.0",
  "region": "us-east-1",
  "cluster": "cluster-prod"
}
```

O gateway **loga todos os metadados** a cada mensagem recebida:

```go
log.Printf("health request caller_id=%q peer_ip=%q peer_addr=%q cert_name=%q metadata=%v",
    callerID, peerIP, peerAddr, certName, req.GetMetadata())
```

> **Importante**: `caller_id` é o identificador lógico (não autenticado). O `cert_name` extraído do certificado mTLS é o identificador **confiável** — porque é emitido pelo CA controlado pelo `kdctl init`.

### Estado em Memória (por conexão ativa)

```go
type healthService struct {
    mu              sync.RWMutex

    // última request recebida, indexada por caller_id
    lastByCaller    map[string]*health.HealthCheckRequest

    // clientes ativos: caller_id → HealthClient{Id, Name, PeerIP}
    clientConnected map[string]*health.HealthClient
}
```

- Quando o stream **abre**: `clientConnected[callerID]` é populado com `Id`, `Name` e `PeerIP`.
- Quando o stream **fecha** (`defer`): a entrada é removida de ambos os mapas.
- `sync.RWMutex` protege leituras concorrentes (`HealthClientStream`) e escritas (`HealthStream`).

---

## Fluxo: `HealthStream` (bidirecional)

```
Cliente abre stream
    │
    └─ loop Recv()
            │
            ├─ msg.GetRequest() == nil → descarta e continua
            │
            └─ msg.GetRequest() válido
                    │
                    ├─ extrai callerID = req.GetCallerId()
                    ├─ extrai peerAddr + peerIP de peer.FromContext()
                    ├─ extrai certName do certificado mTLS (URI > DNS > CN)
                    ├─ registra em lastByCaller[callerID] e clientConnected[callerID]
                    ├─ loga: caller_id, peer_ip, peer_addr, cert_name, metadata
                    │
                    └─ envia HealthCheckResponse:
                            request_id   = req.GetId()
                            responder_id = "kd-gateway"
                            responded_at = now()
                            next_check_in = 10s
                            checks[0] = { name:"overall", code:UP, message:"received" }

Cliente fecha stream (EOF ou erro)
    └─ defer: remove callerID de lastByCaller e clientConnected
```

---

## Fluxo: `HealthClientStream` (server-side stream)

```
Cliente envia HealthClientRequest
    │
    └─ servidor lê clientConnected (com RLock)
            │
            └─ monta slice de HealthClient para todos os callerIDs ativos
                    │
                    └─ envia HealthClientResponse:
                            request_id   = req.GetId()
                            responder_id = "kd-gateway"
                            clients[]    = todos os clientes conectados
```

---

## Segurança: mTLS

| Modo | Configuração | Comportamento |
|---|---|---|
| **mTLS habilitado** (padrão) | `GRPC_MTLS=1` + `GRPC_CLIENT_CA_FILE=<path>` | Servidor exige e valida o certificado do cliente (`tls.RequireAndVerifyClientCert`) |
| **TLS simples** | `GRPC_MTLS=0` | Apenas o certificado do servidor é usado; `cert_name` ficará vazio |

O CA do cliente (`GRPC_CLIENT_CA_FILE`) deve ser o mesmo `ca.crt` gerado pelo `kdctl init` — garantindo que apenas agentes provisionados pelo sistema podem se conectar.

---

## Variáveis de Ambiente

| Variável | Padrão (main.go) | Descrição |
|---|---|---|
| `GRPC_MTLS` | `1` | Habilita mTLS (`1`/`true`) ou desabilita (`0`/`false`) |
| `GRPC_CLIENT_CA_FILE` | `~/.kubediscovery/certs/staging/ca.crt` | CA para validar certificados dos clientes |
| `GRPC_CERT_FILE` | configurado no `Config` struct | Certificado TLS do servidor |
| `GRPC_KEY_FILE` | configurado no `Config` struct | Chave privada do servidor |
| `GRPC_DEBUG` | `""` (desabilitado) | Ativa interceptors extras de debug com duração por chamada |

---

## Interceptors de Log

### Sempre ativos

| Interceptor | Tipo | O que loga |
|---|---|---|
| `UnaryInterceptors` | Unary | `method`, `request`, `server` |
| `StreamInterceptors` | Stream | `method`, `peer`, `client_stream`, `server_stream`, duração, erro |

### Ativos apenas com `GRPC_DEBUG=1`

| Interceptor | Tipo | O que loga |
|---|---|---|
| `unaryDebugInterceptor` | Unary | `method`, `peer`, duração, erro |
| `streamDebugInterceptor` | Stream | `method`, `peer`, `client_stream`, `server_stream`, duração, erro |

---

## Arquivos do Módulo

| Arquivo | Responsabilidade |
|---|---|
| `cmd/grpc/main.go` | Entry point: configura mTLS, instancia `grpcserver.NewServer`, registra `healthService`, inicia o servidor |
| `pkg/grpcserver/grpc.go` | `NewServer`: cria listener, configura TLS/mTLS, encadeia interceptors, retorna `*GRPC` |
| `internal/infrastructure/middleware/grpc/interceptor.go` | `UnaryInterceptors` e `StreamInterceptors` (log + peer) sempre ativos |
| `libs/core/v1/proto/health.proto` | Contrato gRPC: define `HealthService`, todas as mensagens e `HealthStatusCode` |
| `libs/core/v1/health/health.pb.go` | Código Go gerado pelo protoc (mensagens) |
| `libs/core/v1/health/health_grpc.pb.go` | Código Go gerado pelo protoc (server/client stubs) |

---

## Bibliotecas Utilizadas

| Biblioteca | Versão | Propósito |
|---|---|---|
| `google.golang.org/grpc` | v1.80.0 | Framework gRPC — servidor, streaming, credentials, peer, interceptors |
| `google.golang.org/protobuf` | v1.36.11 | Runtime protobuf — `timestamppb`, `durationpb`, `structpb` |
| `github.com/kubediscovery/kd-libs` | local (`../../libs`) | Definições proto compiladas (`health.pb.go`, `health_grpc.pb.go`) | 