# KUBEDISCOVERY EXECUTOR (KD-EXECUTOR)

## Overview

O `kd-executor` é o módulo de **execução e observabilidade** do Data Plane Kubediscovery. Ele roda dentro do cluster remoto (junto ao `kd-agent`) e possui duas responsabilidades complementares:

| Componente | Função |
|---|---|
| **WATCHER** | Operator que monitora eventos, logs e status de recursos Kubernetes em tempo real e alimenta o `kd-analyzer` |
| **EXECUTOR** | Motor de execução que aplica ações no cluster — comandos vindos do `kd-analyzer` (via `kd-gateway`) ou requisições externas diretas |

> **Habilitação**: ambos os componentes são controlados pelo manifesto do Kubernetes Operator (`Agent CRD`), podendo ser ligados/desligados independentemente sem reiniciar o `kd-agent`.

```yaml
apiVersion: kubediscovery.io/v1beta1
kind: Agent
metadata:
  name: agent-srv001
spec:
  agent:
    enabled: true
  executor:
    enabled: true   # habilita EXECUTOR
    watcher: true   # habilita apenas o WATCHER se o executor estiver ativo.  Se o executor estiver desabilitado, o watcher não deve funcionar, pois é uma funcionalidade do executor
    enabled: false
```

---

## Arquitetura

```
Cluster Remoto (Data Plane)
┌─────────────────────────────────────────────────────────────────┐
│                                                                 │
│   ┌─────────────────┐        ┌────────────────────────────┐    │
│   │    WATCHER      │        │        EXECUTOR            │    │
│   │                 │        │                            │    │
│   │ client-go       │        │  Executor interface (Go)   │    │
│   │ informers:      │        │  ┌──────────────────────┐  │    │
│   │ - Pod           │        │  │ ScaleAction          │  │    │
│   │ - Node          │        │  │ RestartAction        │  │    │
│   │ - Ingress       │        │  │ PatchResourceAction  │  │    │
│   │ - Events        │        │  │ ExecAction           │  │    │
│   │                 │        │  │ LogsAction           │  │    │
│   │ Detecta:        │        │  │ DescribeAction       │  │    │
│   │ OOMKilled       │        │  └──────────────────────┘  │    │
│   │ CrashLoopBack   │        │                            │    │
│   │ Pending+taint   │        └──────────┬─────────────────┘    │
│   │ FailedSchedule  │                   │ aplica via            │
│   │ Warning Events  │                   │ client-go / kubectl   │
│   └────────┬────────┘                   │                      │
│            │ envia dados                │ ExecutorResult        │
│            │ (logs, events,             │                       │
│            │  describe)                 │                       │
└────────────┼───────────────────────────┼──────────────────────-┘
             │                           │
             │ gRPC bidirecional (mTLS)   │ gRPC bidirecional (mTLS)
             ▼                           ▲
         kd-agent  ──────────────────────┘
             │
             │ gRPC bidirecional (mTLS)
             ▼
         kd-gateway
             │
             ├─── kd-analyzer (recomenda ação)
             │         └─── retorna AnalysisResult.Resolver
             │
             └─── kd-executor recebe ExecutorAction → executa no cluster
```

---

## Componente 1: WATCHER

### Responsabilidade

O WATCHER é um **Kubernetes controller** (usando `client-go` informers) que observa continuamente o estado do cluster e detecta condições anômalas. Quando detecta um evento relevante, coleta os dados necessários (logs, describe, events) e os envia ao `kd-gateway`, que os encaminha ao `kd-analyzer`.

### Recursos Monitorados

| Recurso | O que monitora | Condições de trigger |
|---|---|---|
| **Pod** | Status, restart count, container state | `OOMKilled`, `CrashLoopBackOff`, `Error`, `Pending` |
| **Pod Events** | Events do namespace | `FailedScheduling`, `BackOff`, `Unhealthy`, `OOMKilling` |
| **Node** | Condições (Ready, MemoryPressure, DiskPressure) | `NotReady`, `MemoryPressure`, `DiskPressure`, `PIDPressure` |
| **Node Events** | Events do node | `Eviction`, `NodeNotReady`, taint events |
| **Ingress** | Status, backend health | Ingress sem Address, backend com erro |
| **Karpenter** (condicional) | Logs do deployment `karpenter` | Pod Pending com `untolerated taint`, `0/N nodes available` |

### Dados coletados por evento

Quando um trigger é detectado, o WATCHER coleta e empacota:

```go
type WatcherPayload struct {
    // Identificação
    ClusterName  string    `json:"cluster_name"`
    Environment  string    `json:"environment"`
    Namespace    string    `json:"namespace"`
    ResourceKind string    `json:"resource_kind"` // Pod | Node | Ingress
    ResourceName string    `json:"resource_name"`
    TriggerEvent string    `json:"trigger_event"` // OOMKilled | CrashLoopBackOff | Pending | ...

    // Dados coletados
    Logs        string    `json:"logs"`         // últimas N linhas de log do container
    Events      string    `json:"events"`       // kubectl get events -n <ns> --field-selector involvedObject.name=<pod>
    Describe    string    `json:"describe"`     // kubectl describe <kind> <name> -n <ns>
    KarpenterLogs string  `json:"karpenter_logs,omitempty"` // coletado se trigger for Pending+taint

    // Contexto
    DetectedAt  time.Time `json:"detected_at"`
    Source      string    `json:"source"` // "watcher"
}
```

### Fluxo do WATCHER

```
client-go informer detecta mudança de estado
    │
    ├─ Pod.Status.ContainerStatuses[].LastTerminationState.Terminated.Reason == "OOMKilled"?
    ├─ Pod.Status.Phase == "Pending" por mais de N segundos?
    ├─ Pod.Status.ContainerStatuses[].RestartCount > threshold?
    ├─ Event.Reason == "FailedScheduling" | "BackOff" | "Unhealthy"?
    ├─ Node.Status.Conditions[].Type == "MemoryPressure" | "NotReady"?
    │
    └─ Trigger identificado → coleta:
            ├─ kubectl logs <pod> -n <ns> --tail=200
            ├─ kubectl describe <kind> <name> -n <ns>
            ├─ kubectl get events -n <ns> --field-selector involvedObject.name=<name>
            └─ [se Pending + taint] kubectl logs -n karpenter deployment/karpenter --tail=200
                    │
                    └─ monta WatcherPayload → envia via gRPC ao kd-agent
                            → kd-agent → kd-gateway → kd-analyzer
```

---

## Componente 2: EXECUTOR

### Responsabilidade

O EXECUTOR recebe `ExecutorAction` e aplica as ações no cluster via `client-go`. As ações chegam de duas origens:

| Origem | Fluxo |
|---|---|
| **kd-analyzer** | WATCHER detecta → kd-analyzer analisa → retorna `AnalysisResult.Resolver` → kd-gateway → kd-executor executa |
| **Externo** (usuário/MCP) | Usuário → MCP → kd-gateway → kd-analyzer processa → kd-executor executa |

### Interface Go

```go
// Executor é a interface central do módulo de execução.
// Qualquer nova ação deve implementar esta interface.
type Executor interface {
    Execute(ctx context.Context, action ExecutorAction) (ExecutorResult, error)
    ActionType() string
}

// ExecutorAction é o comando recebido pelo EXECUTOR.
type ExecutorAction struct {
    // Identificação da ação
    ActionType   string            `json:"action_type"`   // scale | restart | patch | exec | logs | describe | karpenter_logs
    Source       string            `json:"source"`        // "analyzer" | "external"
    AnalysisRef  string            `json:"analysis_ref"`  // ID do AnalysisResult que gerou esta ação (se source=analyzer)

    // Contexto do recurso
    ClusterName  string            `json:"cluster_name"`
    Environment  string            `json:"environment"`
    Namespace    string            `json:"namespace"`
    ResourceKind string            `json:"resource_kind"` // Deployment | Pod | Node | Ingress | ...
    ResourceName string            `json:"resource_name"`

    // Parâmetros específicos da ação
    Params       map[string]string `json:"params"`
    // Exemplos:
    //   scale:    {"replicas": "3"}
    //   patch:    {"resources.limits.memory": "512Mi"}
    //   exec:     {"command": "kill -9 1"}
    //   logs:     {"tail": "200", "container": "app"}
}

// ExecutorResult é o retorno de toda execução.
type ExecutorResult struct {
    ActionType  string    `json:"action_type"`
    Success     bool      `json:"success"`
    Output      string    `json:"output"`      // stdout do comando
    Error       string    `json:"error,omitempty"`
    ExecutedAt  time.Time `json:"executed_at"`

    // Correlação com o ciclo analyzer → executor
    AnalysisRef string    `json:"analysis_ref,omitempty"`
    ClusterName string    `json:"cluster_name"`
    Namespace   string    `json:"namespace"`
    ResourceKind string   `json:"resource_kind"`
    ResourceName string   `json:"resource_name"`
}
```

### Ações Suportadas

| `ActionType` | Params | Operação Kubernetes |
|---|---|---|
| `scale` | `replicas` | `kubectl scale deployment/<name> --replicas=N` |
| `restart` | — | `kubectl rollout restart deployment/<name>` |
| `patch_resources` | `cpu_limit`, `memory_limit`, `cpu_request`, `memory_request` | `kubectl patch pod/<name> -p {...}` |
| `cordon_node` | — | `kubectl cordon <node>` |
| `uncordon_node` | — | `kubectl uncordon <node>` |
| `drain_node` | `grace_period` | `kubectl drain <node> --ignore-daemonsets --delete-emptydir-data` |
| `exec` | `container`, `command` | `kubectl exec <pod> -c <container> -- <command>` |
| `logs` | `tail`, `container`, `previous` | `kubectl logs <pod> --tail=N` |
| `describe` | — | `kubectl describe <kind> <name> -n <ns>` |
| `karpenter_logs` | `tail` | `kubectl logs -n karpenter deployment/karpenter --tail=N` |
| `delete_pod` | — | `kubectl delete pod/<name> -n <ns>` |
| `apply_manifest` | `manifest` (YAML inline) | `kubectl apply -f -` |

---

## Fluxo Completo: WATCHER → ANALYZER → EXECUTOR

```
WATCHER detecta OOMKilled no pod "api-server-xyz" em production/payments
    │
    ├─ coleta: logs (200 linhas), describe, events
    └─ WatcherPayload → kd-agent → kd-gateway
                                        │
                                        └─ kd-gateway decide: acionar kd-analyzer
                                                │
                                                └─ kd-analyzer executa pipeline:
                                                        ├─ log_analyst    → OOM por memory limit baixo
                                                        ├─ event_analyst  → padrão recorrente (3x em 1h)
                                                        ├─ describe_analyst → limits: memory=256Mi
                                                        └─ remedy_advisor →
                                                                AnalysisResult{
                                                                  Request:  "OOMKilled api-server-xyz",
                                                                  Analyze:  "Memory limit de 256Mi insuficiente. Pico de uso: 312Mi.",
                                                                  Resolver: "patch_resources memory_limit=512Mi"
                                                                }
                                                                        │
                                                                        └─ kd-gateway → kd-executor
                                                                                │
                                                                                └─ ExecutorAction{
                                                                                      ActionType:   "patch_resources",
                                                                                      Namespace:    "payments",
                                                                                      ResourceKind: "Pod",
                                                                                      ResourceName: "api-server-xyz",
                                                                                      Params: {
                                                                                        "memory_limit": "512Mi"
                                                                                      }
                                                                                   }
                                                                                        │
                                                                                        └─ kubectl patch → ExecutorResult{Success: true}
                                                                                                │
                                                                                                └─ kd-store persiste AnalysisResult + ExecutorResult
```

## Fluxo Completo: Requisição Externa (Usuário)

```
Usuário → MCP → kd-gateway
    │
    └─ payload: "scale deployment api-server para 5 replicas em payments/production"
            │
            └─ kd-gateway → kd-analyzer (valida e processa request)
                    │
                    └─ remedy_advisor → AnalysisResult{
                                          Resolver: "scale replicas=5"
                                       }
                            │
                            └─ kd-gateway → kd-executor
                                    │
                                    └─ ExecutorAction{
                                          ActionType:   "scale",
                                          ResourceKind: "Deployment",
                                          ResourceName: "api-server",
                                          Namespace:    "payments",
                                          Params: {"replicas": "5"}
                                       }
                                            │
                                            └─ kubectl scale → ExecutorResult{Success: true}
                                                    │
                                                    └─ kd-gateway → responde ao usuário
```

---

## Estrutura de Diretórios

```
kd-executor/
├── cmd/
│   └── grpc/
│       └── main.go                 # entry point — conecta ao kd-agent via gRPC
│
├── internal/
│   ├── core/
│   │   ├── executor/
│   │   │   ├── interface.go        # Executor interface + ExecutorAction + ExecutorResult
│   │   │   ├── scale.go            # ScaleAction implements Executor
│   │   │   ├── restart.go          # RestartAction implements Executor
│   │   │   ├── patch_resources.go  # PatchResourcesAction implements Executor
│   │   │   ├── cordon_node.go      # CordonNodeAction implements Executor
│   │   │   ├── drain_node.go       # DrainNodeAction implements Executor
│   │   │   ├── exec.go             # ExecAction implements Executor
│   │   │   ├── logs.go             # LogsAction implements Executor
│   │   │   ├── describe.go         # DescribeAction implements Executor
│   │   │   ├── delete_pod.go       # DeletePodAction implements Executor
│   │   │   ├── apply_manifest.go   # ApplyManifestAction implements Executor
│   │   │   ├── karpenter_logs.go   # KarpenterLogsAction implements Executor
│   │   │   └── registry.go         # map[ActionType]Executor — lookup por string
│   │   │
│   │   └── watcher/
│   │       ├── watcher.go          # controller principal — start/stop informers
│   │       ├── pod_handler.go      # OnAdd/OnUpdate/OnDelete para Pods
│   │       ├── node_handler.go     # OnAdd/OnUpdate/OnDelete para Nodes
│   │       ├── ingress_handler.go  # OnAdd/OnUpdate/OnDelete para Ingresses
│   │       ├── event_handler.go    # OnAdd para Events (Warning filter)
│   │       ├── trigger.go          # lógica de decisão: condição → WatcherPayload
│   │       └── collector.go        # coleta logs, describe, events via client-go
│   │
│   └── infrastructure/
│       ├── k8s/
│       │   ├── client.go           # cria *kubernetes.Clientset (in-cluster ou kubeconfig)
│       │   └── informer_factory.go # SharedInformerFactory com resync period
│       └── grpc/
│           └── client.go           # cliente gRPC para enviar WatcherPayload ao kd-agent
│
└── go.mod
```

---

## Registro de Executores (`registry.go`)

O `registry` é um mapa `ActionType → Executor` que permite adicionar novas ações sem modificar o dispatcher:

```go
// Registry mapeia ActionType para a implementação correta.
type Registry struct {
    executors map[string]Executor
}

func NewRegistry(client *kubernetes.Clientset) *Registry {
    r := &Registry{executors: make(map[string]Executor)}
    r.Register(NewScaleAction(client))
    r.Register(NewRestartAction(client))
    r.Register(NewPatchResourcesAction(client))
    r.Register(NewCordonNodeAction(client))
    r.Register(NewDrainNodeAction(client))
    r.Register(NewExecAction(client))
    r.Register(NewLogsAction(client))
    r.Register(NewDescribeAction(client))
    r.Register(NewDeletePodAction(client))
    r.Register(NewKarpenterLogsAction(client))
    return r
}

func (r *Registry) Dispatch(ctx context.Context, action ExecutorAction) (ExecutorResult, error) {
    exec, ok := r.executors[action.ActionType]
    if !ok {
        return ExecutorResult{}, fmt.Errorf("unknown action type: %s", action.ActionType)
    }
    return exec.Execute(ctx, action)
}
```

---

## Variáveis de Ambiente

| Variável | Padrão | Descrição |
|---|---|---|
| `KD_EXECUTOR_ENABLED` | `true` | Liga/desliga o EXECUTOR (sobrescrito pelo CRD) |
| `KD_WATCHER_ENABLED` | `true` | Liga/desliga o WATCHER (sobrescrito pelo CRD) |
| `KD_WATCHER_NAMESPACES` | `""` (todos) | Namespaces monitorados (separados por vírgula) |
| `KD_WATCHER_RESYNC_PERIOD` | `30s` | Período de resync dos informers |
| `KD_WATCHER_POD_RESTART_THRESHOLD` | `3` | RestartCount mínimo para trigger |
| `KD_WATCHER_PENDING_THRESHOLD` | `120s` | Tempo máximo em Pending antes de trigger |
| `KD_WATCHER_LOG_TAIL_LINES` | `200` | Linhas de log coletadas por trigger |
| `KD_AGENT_ADDR` | `localhost:50052` | Endereço gRPC do kd-agent para envio de payloads |
| `KUBECONFIG` | `""` (in-cluster) | Path do kubeconfig (dev local) |

---

## Bibliotecas Utilizadas

| Biblioteca | Propósito |
|---|---|
| `k8s.io/client-go` | Kubernetes client, informers, dynamic client, exec |
| `k8s.io/api` | Tipos Kubernetes (Pod, Node, Ingress, Event) |
| `k8s.io/apimachinery` | Types auxiliares (ObjectMeta, LabelSelector) |
| `google.golang.org/grpc` | Comunicação gRPC com kd-agent |
| `google.golang.org/protobuf` | Serialização das mensagens WatcherPayload / ExecutorAction |
