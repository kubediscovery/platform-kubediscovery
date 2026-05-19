# Proposta de Projeto: Plataforma Kubediscovery

## 1. Resumo Executivo
O **Kubediscovery** é uma plataforma de gerenciamento e orquestração distribuída para ambientes Kubernetes. O objetivo principal é estabelecer uma arquitetura centralizada (Control Plane / Server) capaz de gerenciar e interagir de forma transparente e segura com múltiplos clusters remotos (Data Plane / Clients) através de Kubernetes Operators.

A plataforma permitirá não apenas a execução remota de comandos administrativos nos clusters, mas também o provisionamento de análises avançadas baseadas em Inteligência Artificial (LLMs) diretamente no ecossistema Kubernetes.

Tenho uma ideia que o **Data Plane (Client)**, pode ser um Kubernetes Controler, onde irá gerenciar as instâncias do kd-agent, executor e analyzer. Podendo habilitar ou desabilitar o recurso.

Exemplo do Operator:
```yaml
apiVersion: kubediscovery.io/v1beta1
kind: Agent
metadata:
  name: agent-srv001
spec:
  agent:
    enabled: true
  executor:
    enabled: true
  analyzer:
    enabled: false
  troubleshootingImage:
    enabled: false
```

Com isso, irá criar as instanĉias de cada servićo que se conectará no kd-agent, que por sua vez conecta no gateway.


**Fluxo da Request pelo usuário**:
USER requester > MCP > gateway (decide o servidor) > enviar ao kd-agent > kd-agent decide a action > action de executor > retornat ao kd-agent > retornar kd-gateway > retonar ao user

**Fluxo identificaćão de problemas**:
kd-executor (watcher) > kd-agent > kd-gateway (analyzer) > notificaćão


## 2. Visão Geral da Arquitetura
A arquitetura proposta baseia-se em uma topologia Server-Client, onde a comunicação é estritamente realizada via **gRPC bidirecional** com troca de certificado autoassinado, garantindo alta performance, baixa latência e segurança robusta.

A plataforma é composta por quatro pilares principais:

### 2.1. CLI (Command Line Interface) - `kdctl`
A porta de entrada operacional para administração da plataforma (inspirada na usabilidade do `kubectl`).
* **Responsabilidades:**
    * Inicialização e configuração do projeto.
    * Criação, registro e listagem de clusters clientes.
    * Geração e gerenciamento de certificados mTLS (autoassinados) para comunicação segura de cada cluster.
    * Emissão de comandos de gerenciamento do ciclo de vida, como ações de pausa, registro e remoção de clientes baseando-se em UIDs.
    * Listar clusters conectados no control plane
    * Consultar histórico de valićões e problemas

### 2.2. Control Plane (Server / Core)
O núcleo central da plataforma, hospedado em um cluster de gerência. Ele orquestra todas as conexões, mantém o estado global da infraestrutura e centraliza o processamento principal.
**ATTENTION** toda a comunicaćão entre control plane e data plane, deverá ser via gRPC bidirecional com mTLS.
* 
* **Componentes:**
    * **kd-gateway:** Ponto central de entrada gRPC para todas as conexões dos clientes, responsável pelo roteamento e orquestração de tarefas e conexões.
    * **kd-analyzer (Executor LLM):** Módulo inteligente encarregado de receber requisições, processar dados dos clusters e fornecer insights operacionais através de Modelos de Linguagem (LLM).
    * **kd-executor:** Responsável por executar comandos administrativos no cluster principal.
    * **kd-store (Database):** Persistência de registros, configurações e informações de estado dos clusters.  Além disso, o kd-store servirá como memory dos eventos.  Onde numa próxima analise irá consultar o histórico da aplicaćão.   Exemplo do indice/sessão e ser buscada para o histórioco:
        clusterName+Environment+NameSpace
    * **kd-cache (Redis):** Gerenciamento de estado e acesso rápido a dados em memória, garantindo alta performance de execução.

### 2.3. Data Plane (Client)
Operators instalados nos clusters remotos, projetados para receber instruções do Control Plane e executá-las localmente.

**ATTENTION** toda a comunicaćão entre control plane e data plane, deverá ser via gRPC bidirecional com mTLS.

* **Componentes:**
    * **kd-agent:** Serviço executado nos clusters remotos, encarregado de iniciar e manter a conexão gRPC bidirecional com o `kd-gateway` (Server).
    * **kd-agent-executor:** Encarregado de traduzir e executar comandos Kubernetes recebidos do servidor no cluster local.
    * **kd-agent-analyzer (LLM Client):** Um módulo com escopo reduzido para realizar análises via LLM localmente no próprio cluster cliente. Isso permite inteligência na ponta, mitigando o envio de excesso de dados para o Server.

### 2.4. Dashboard (Portal)
A interface gráfica de administração e operação.
* **Componentes:**
    * **kd-portal:** Painel de controle Web interativo para visualização centralizada do estado, métricas de saúde (health) de todos os clusters registrados, e execução simplificada das operações disponíveis na plataforma.

## 3. Dinâmica de Registro e Comunicação
A segurança e resiliência são o cerne da comunicação do Kubediscovery:
1. **Provisionamento:** Utilizando o `kdctl`, o administrador provisiona um novo cliente enviando nome e ambiente, gerando um UID e certificados únicos. O status inicial é definido como "unregistered".
2. **Registro e Healthcheck:** O Control Plane manterá vigilância sobre os clientes provisionados. No primeiro contato ativo do cliente validado (registro), estabelece-se um fluxo de **healthcheck bidirecional** constante.
3. **Gestão do Ciclo de Vida:** O servidor gerencia o acesso do cliente permitindo bloqueios temporários (via comando `action: pause`) ou sua remoção definitiva através do seu UID.
4. **Execução Sob Demanda:** Através do túnel gRPC ativo, o servidor pode solicitar operações assíncronas e síncronas diretamente no cluster alvo, como operações nativas de Kubernetes ou processamento cognitivo por IA.
5. **Client Request** O client pode enviar operaćões para o servidor, dependendo da congiguraćão aplicada pelo o usuário
6. **AI Tokens** Os tokens serão gerenciados pelo o server, podem serem repassados aos Data Plane (Client) conforme a configuraćão.  O Data Plane (Client) pode executar o LLm local através de uma instância de POD ou pode enviar ao Control Plane (server) executar.  Essa configuraćão dependerá de como for configurado o cliente

## 4. Gestão de Acesso e Permissionamento (IAM)
Para garantir que os usuários e sistemas executem apenas ações autorizadas, a plataforma adotará um modelo de autorização refinado (Fine-Grained Access Control).

O permissionamento será estruturado mimetizando o modelo de **RBAC do Kubernetes**, estendido para suportar os recursos exclusivos da plataforma:
* **Escopos de K8s:** Controle baseado na tripla clássica: `Verbs` (ex: get, list, create, delete, exec), `Namespaces` e `Kinds` (ex: Pods, Deployments).
* **Escopos de IA:** Permissões adicionais para o uso do LLM (ex: `llm:analyze`, `llm:query`), permitindo restringir quais usuários ou tenants podem consumir processamento de IA.
* **Escopos de Plataforma:** Controle sobre os clusters registrados (ex: `cluster:pause`, `cluster:register`, `cluster:delete`).

### Recomendação de Tecnologia: OPA (Open Policy Agent)
Recomendamos fortemente a utilização do **OPA (Open Policy Agent)** para gerenciar o permissionamento.
* **Por que OPA?**
    * É o padrão global da CNCF para políticas em ambientes Cloud Native.
    * Sua linguagem de políticas (**Rego**) é perfeita para avaliar regras baseadas em atributos (ABAC), avaliando exatamente o seu caso de uso: *"O usuário X tem permissão para usar o verbo Y no kind Z dentro do namespace W no cluster C?"*
    * Pode ser embutido como uma biblioteca Go no seu `kd-gateway` ou rodar como um sidecar. Assim, o gateway intercepta a chamada gRPC, pede a validação pro OPA e só encaminha para o Client se o OPA retornar `allow = true`.
* **Sobre o OpenFGA:** O OpenFGA (baseado em Zanzibar) é excelente para *ReBAC* (Relationship-Based Access Control), onde as permissões são herdadas por relações hierárquicas complexas ("Usuário é membro do time A, que é dono da pasta B, que contém o arquivo C"). Para uma plataforma de infraestrutura baseada em recursos Kubernetes (verbos, kinds, namespaces), as políticas do **OPA** oferecem um encaixe técnico mais natural, mais flexível para o modelo Kubernetes e são mais familiares para as equipes de DevOps.

## 5. Benefícios e Valor para o Negócio
* **Gestão Centralizada (Single Pane of Glass):** Visão unificada e capacidade de gerir dezenas ou centenas de clusters a partir de um único ponto.
* **Operações Inteligentes (AIOps):** Integração nativa da administração Kubernetes com LLMs para facilitação de troubleshooting, análise de logs e provimento de insights diretos, aumentando a eficiência da equipe de DevOps.
* **Segurança Restrita e Simplificada:** Comunicação nativamente criptografada em modelo gRPC, com validação de certificados mTLS por cliente e autorização centralizada de acessos via OPA.
* **Escalabilidade Elevada:** Arquitetura orientada a microsserviços bem delimitados, utilizando Redis para cache e processamento assíncrono para os agentes.

## 6. Próximos Passos
1. Finalizar e validar as funcionalidades core do **kdctl** (gestão de certificados, listagens e UIDs) já em desenvolvimento.
2. Implementar o Minimum Viable Product (MVP) do **kd-gateway** no Server e **kd-agent** no Client para validar o fluxo de rede gRPC bidirecional.
3. Estruturar os schemas do banco de dados (kd-store) e instanciar o cache Redis (kd-cache).
4. Definir a matriz de políticas (Rego) iniciais no OPA para validar o permissionamento e segurança da execução remota.
5. Desenvolver o módulo primário de execução remota de comandos de Kubernetes.
6. Iniciar Prova de Conceito (PoC) do componente **kd-analyzer** para processamento de IA/LLM.

## 6. Estrutura de diretórios
- **.src/backend/libs**: libs a serem compartilhadas com todos os servićos Golang
- **.src/backend/directoryServiceName**: servićos desenvolvidos em golang que são control plane **kd-gateway**, data plane **kd-agent** e a CLI que é **kdctl**
- **frontend**: o frontend do projeto que tem o dashboard
- **cli**: será desenvolvida a partir do golang cobra
- **config**:  as configuraćões serão no padrão do spf13-viper. Tendo o path de configuraćão padrão como ~/.kubedicovery se for CLI ou variáveis de ambiente se for deploy dos servićos
- **depedency injectio**: usaremos o UberFX para o gerenciamento de dependency injection
- **Estrutura proposta para cada service**:
- **observability**: todos os servićos precisam trabalhar com o Prometheus e OpenTelemtry com tracers
  - Prometheus em casos de API tem que ter as metrics
  - Tracer, precisam iniciar nas request e origem de cada dado
  - logger
```sh 
.
├── cmd
│   └── app
│       ├── main.go
│       ├── setup.go          # bootstrap do Uber FX + graceful shutdown
│       ├── providers.go      # providers globais
│       └── types.go          # tipos auxiliares do FX
│
├── configs
│   ├── config.go             # config raiz
│   ├── grpc.go
│   ├── http.go
│   ├── database.go
│   ├── cache.go
│   └── llm.go
│
├── internal
│   ├── core
│   │   ├── cluster
│   │   │   ├── entity
│   │   │   │   └── cluster.go
│   │   │   ├── service
│   │   │   │   └── cluster_service.go
│   │   │   ├── repository
│   │   │   │   ├── cluster_repository.go
│   │   │   │   └── postgres_repository.go
│   │   │   ├── handler
│   │   │   │   ├── http_handler.go
│   │   │   │   └── grpc_handler.go
│   │   │   └── module.go
│   │   │
│   │   └── discovery
│   │       ├── entity
│   │       ├── service
│   │       ├── repository
│   │       ├── handler
│   │       └── module.go
│   │
│   └── infrastructure
│       ├── http
│       │   ├── server.go
│       │   ├── router.go
│       │   └── middleware
│       │       ├── auth.go
│       │       ├── logger.go
│       │       └── recovery.go
│       │
│       ├── grpc
│       │   └── server.go
│       │
│       ├── database
│       │   ├── postgres.go
│       │   ├── migrations.go
│       │   └── transaction.go
│       │
│       ├── cache
│       │   └── redis.go
│       │
│       ├── llm
│       │   ├── client.go
│       │   ├── databricks.go
│       │   └── openai.go
│       │
│       └── kubernetes
│           ├── client.go
│           ├── informer.go
│           └── watcher.go
│
├── pkg
│   ├── logger
│   ├── errors
│   ├── validator
│   └── response
│
├── api
│   ├── proto
│   └── openapi
│
├── migrations
│   └── 000001_create_clusters_table.sql
│
├── build
│   ├── Dockerfile
│   └── docker-compose.yaml
│
├── infra
│   ├── helm
│   ├── k8s
│   └── terraform
│
├── go.mod
├── go.sum
└── README.md
```