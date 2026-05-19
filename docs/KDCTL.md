# KUBEDISCOVERY CONTROL - CLI (KDCTL)

## Overview

`kdctl` is a command-line interface (CLI) tool designed to interact with the Kubediscovery system. With `kdctl`, users can execute a variety of commands to manage and query the cluster state, validate incidents, and gain insights into the health of the Kubernetes environment.

## Funcionalidades

- **Iniciando o projeto**: `kdctl init` - Configura o ambiente de trabalho para usar o Kubediscovery.

---

## Comando: `kdctl init`

### Descrição

O comando `init` inicializa o ambiente de trabalho do `kdctl`. Ele é responsável por:

1. Gerar um **Certificate Authority (CA)** local e os certificados TLS do servidor gRPC.
2. Salvar a configuração do cluster (nome, endereço, porta, ambiente) em `~/.kubediscovery/certs/<environment>/`.
3. Oferecer um fluxo interativo via formulário no terminal quando nenhuma flag obrigatória é fornecida.

---

### Uso

```bash
# Modo interativo (sem flags): abre formulário TUI
kdctl init

# Modo não-interativo: registra cluster diretamente via flags
kdctl init --name <nome> --address <host:porta> --environment <ambiente>
```

#### Flags

| Flag            | Atalho | Padrão          | Descrição                                                        |
|-----------------|--------|-----------------|------------------------------------------------------------------|
| `--name`        | `-n`   | `""`            | Nome único que identifica o servidor Kubediscovery               |
| `--address`     | `-a`   | `localhost:50051`| Endereço (`host` ou `host:porta`) do servidor gRPC              |
| `--environment` | `-e`   | `development`   | Ambiente onde o servidor foi implantado (ex: production, staging) |

> Quando `--name` é fornecido, `--address` e `--environment` tornam-se obrigatórios.

---

### Fluxo de Execução

```
kdctl init
    │
    ├─ [flag --name passada?]
    │       └── Sim → valida address + environment → salva config → encerra
    │
    └─ Não → verifica ~/.kubediscovery/certs/*/ca.crt
                │
                ├─ [certificados existentes?]
                │       └── Sim → exibe lista → pergunta se deseja criar novo CA
                │               └── Não → encerra sem alterações
                │
                └─ Abre formulário interativo (TUI)
                        │
                        ├─ Grupo 1 – Dados do CA
                        │   ├─ Organization Name
                        │   ├─ Country Code (ISO 3166)
                        │   ├─ Province Name
                        │   ├─ City Name
                        │   └─ Domains (separados por vírgula)
                        │
                        ├─ Grupo 2 – Configuração do Servidor
                        │   ├─ Server Name (identificador único)
                        │   ├─ Server Address (IP ou hostname validado)
                        │   ├─ Port (1–65535, padrão 50051)
                        │   └─ Environment
                        │
                        └─ Confirmação → Sim
                                │
                                ├─ Exibe resumo (printSummary)
                                ├─ Spinner: gera CA + certificado do servidor via sslGenerate
                                │   ├─ NewCertificateAuthority(opts) → WriteToFiles(~/.kubediscovery/certs/<env>/)
                                │   └─ NewServerCertificate(name, opts, ca) → WriteToFiles(...)
                                └─ Notificação de sucesso
```

---

### Estrutura de Dados

#### `Cluster`
Representa um servidor Kubediscovery registrado.

```go
type Cluster struct {
    Name    string `json:"name" yaml:"name"`
    Address string `json:"address" yaml:"address"`
    Port    *int   `json:"port" yaml:"port"`
    Env     string `json:"env" yaml:"env"`
}
```

#### `Config`
Arquivo de configuração global persistido em disco.

```go
type Config struct {
    Version  int       `json:"version" yaml:"version"`
    Status   []Update  `json:"status" yaml:"status"`   // histórico dos últimos 10 updates
    Clusters []Cluster `json:"clusters" yaml:"clusters"`
    Context  string    `json:"context" yaml:"context"`
}
```

#### `ServerConfig`
Dados coletados pelo formulário interativo para geração do CA e dos certificados.

```go
type ServerConfig struct {
    OrgName     string   // nome da organização
    Country     string   // código do país (ISO 3166)
    Province    string
    City        string
    Domains     []string // lista de domínios para o certificado
    ServerName  string   // nome único do servidor
    Addr        string   // endereço gRPC
    Port        string   // porta gRPC
    Environment string
    Confirmed   bool     // confirmação do usuário
    HomeDir     string   // caminho base para salvar os certs
}
```

---

### Bibliotecas Utilizadas

| Biblioteca | Versão | Propósito |
|---|---|---|
| `github.com/spf13/cobra` | v1.10.2 | Framework CLI — parsing de flags e subcomandos |
| `charm.land/huh/v2` | v2.0.3 | Formulários TUI interativos (inputs, confirm, note, spinner) |
| `charm.land/lipgloss/v2` | v2.0.1 | Estilização de layout e tema no terminal |
| `charm.land/bubbletea/v2` | v2.0.2 | Engine TUI (Bubble Tea — subjacente ao huh) |
| `github.com/kubediscovery/kd-libs/sslGenerate` | internal | Geração de CA e certificados TLS (x509) |
| `google.golang.org/grpc` | v1.80.0 | Comunicação gRPC com o servidor Kubediscovery |

---

### Artefatos Gerados

Após a execução bem-sucedida do `init`, os seguintes arquivos são criados em `~/.kubediscovery/certs/<environment>/`:

```
~/.kubediscovery/certs/
└── <environment>/
    ├── ca.crt        # Certificado da Autoridade Certificadora
    ├── ca.key        # Chave privada da CA
    ├── server.crt    # Certificado TLS do servidor gRPC
    └── server.key    # Chave privada do servidor
```

---

### Arquivos do Módulo

| Arquivo | Responsabilidade |
|---|---|
| `cmd/initconfig.go` | Define o subcomando `init` no Cobra, registra flags e chama `NewConfiguration` |
| `cmd/init/init.go` | Orquestra o fluxo principal: verifica certs existentes → formulário → spinner → notificação |
| `cmd/init/form.go` | Formulário TUI (`huh`), spinner de geração de certs e funções de layout/summary |
| `cmd/init/types.go` | Tipos `Config`, `Cluster`, `Update`, `ServerConfig` e métodos auxiliares |

---

## Comando: `kdctl server`

### Descrição

O comando `server` agrupa operações de gerenciamento de certificados e conectividade dos servidores Kubediscovery. É um comando pai com subcomandos independentes:

| Subcomando | Descrição |
|---|---|
| `server list` | Lista todos os servidores Kubediscovery registrados |
| `server client` | Conecta-se a um servidor via gRPC, com suporte a modo watch.  Fica aguardando atualizações em tempo real. |

---

### Uso

```bash
# Exibe ajuda do comando pai
kdctl server

# Gerenciar lista registrados conectados e desconectados
kdctl server list   # lista todos os servidores registrados, indicando status (conectado/desconectado)
kdctl server list --connected # lista apenas servidores atualmente conectados
kdctl server list --disconnected # lista apenas servidores atualmente desconectados

# Conectar a um servidor via gRPC
kdctl server client --addr <host:porta>
kdctl server client --addr <host:porta> --watch
```

---

## Subcomando: `kdctl server list`

### Descrição

Lista todos os servidores Kubediscovery registrados, indicando seu status de conexão (conectado/desconectado).
#### Flags

| Flag            | Atalho | Padrão                              | Descrição                                              |
|-----------------|--------|--------------------------------------|--------------------------------------------------------|
| `--all   `      | `-a`   | `false`                              | Lista todos os servidores, incluindo os desconectados |
| `--connected`   | `-c`   | `false`                              | Lista apenas servidores atualmente conectados         |
| `--disconnected`| `-d`   | `false`                              | Lista apenas servidores atualmente desconectados      |
| `--environment` | `-e`   | `""`                                 | Ambiente do servidor (ex: production, staging)         |

### Fluxo de Execução

```
kdctl server list [--all | --connected | --disconnected] [--environment <env>]
    │
    ├─ Carrega configuração de clusters registrados (Config.Clusters)
    ├─ Filtra por ambiente se --environment for fornecido
    ├─ Determina status de conexão para cada cluster (ping gRPC ou cache)
    └─ Exibe tabela formatada no terminal (forms.Table)
```

### Artefatos Gerados

```
# Saída formatada no terminal, exemplo:
+------------------+-------------------+---------+-----------------+
| Name             | Address           | Env     | Status          |
+------------------+-------------------+---------+-----------------+
| kubediscovery-1  | localhost:50051   | staging | Connected       |
| kubediscovery-2  | localhost:50052   | staging | Disconnected    |
| kubediscovery-3  | localhost:50053   | production | Connected    |
+------------------+-------------------+---------+-----------------+
```

---

## Subcomando: `kdctl server client`

### Descrição

Realiza conexão e interação com um servidor Kubediscovery via gRPC. Suporta modo contínuo (`--watch`) para monitoramento em tempo real.

#### Flags

| Flag      | Atalho | Padrão            | Descrição                                      |
|-----------|--------|-------------------|------------------------------------------------|
| `--addr`  | —      | `localhost:50051` | Endereço gRPC do servidor (`host:porta`)       |
| `--watch` | `-w`   | `false`           | Modo contínuo: fica observando o servidor      |

### Fluxo de Execução

```
kdctl server client --addr <host:porta> [--watch]
    │
    └─ Imprime: "Executando server client | addr=<addr> | watch=<bool>"
            └─ [--watch=true] → modo contínuo de observação do servidor
```

---

### Bibliotecas Utilizadas

| Biblioteca | Propósito |
|---|---|
| `github.com/spf13/cobra` | Framework CLI — parsing de flags e subcomandos |
| `github.com/kubediscovery/kd-libs/sslGenerate` | Carregar CA existente e gerar certificados TLS (x509) |
| `github.com/kubediscovery/kd-libs/forms` | Renderização de tabelas (`forms.Table`) e notificações (`forms.Notification`) no terminal |
| `github.com/kubediscovery/kdctl/internal/service/server` | Serviço interno: listagem de certificados via glob em `~/.kubediscovery/certs/` |
| `google.golang.org/grpc` | Comunicação gRPC com o servidor Kubediscovery |

---

### Arquivos do Módulo

| Arquivo | Responsabilidade |
|---|---|
| `cmd/server.go` | Define o comando pai `server`, registra flags globais e adiciona subcomandos |
| `cmd/server/list.go` | Subcomando `list`: listagem de servidores Kubediscovery registrados |
| `cmd/server/client.go` | Subcomando `client`: conexão gRPC ao servidor com suporte a `--watch` |
| `internal/service/server/certificate.go` | `ListAllCertificates()`: percorre `~/.kubediscovery/certs/*/*.crt` e retorna mapa `env → paths` |

---

## Comando: `kdctl certificate`

### Descrição

O comando `certificate` gerencia certificados TLS para **servidores** e **clientes** do ecossistema Kubediscovery. Ele reutiliza o CA criado pelo `kdctl init` para emitir novos certificados assinados.

A diferença central em relação ao `kdctl server certificate` é a presença do flag `--client`, que instrui o comando a gerar um **certificado de cliente** (usado pelo `kd-agent` para autenticação mTLS no `kd-gateway`).

| Operação | Flag | Tipo de cert gerado |
|---|---|---|
| Criar certificado de servidor | `--create --name <nome>` | `sslGenerate.NewServerCertificate` |
| Criar certificado de cliente | `--create --name <nome> --client` | `sslGenerate.NewClientCertificate` |
| Auto-detectar cliente | `--create --name client-<algo>` | `sslGenerate.NewClientCertificate` (pelo prefixo) |
| Listar todos os certificados | `--list` | — |

---

### Uso

```bash
# Criar certificado de SERVIDOR
kdctl certificate --create --name <nome> --environment <env>

# Criar certificado de CLIENTE (kd-agent)
kdctl certificate --create --name <nome> --environment <env> --client

# Auto-detecção de cliente pelo prefixo do nome
kdctl certificate --create --name client-node42 --environment staging

# Especificar diretório e validade
kdctl certificate --create --name srv-prod --environment production --directory /custom/path --year 5

# Listar todos os certificados existentes
kdctl certificate --list
```

---

### Flags

| Flag          | Atalho | Padrão | Descrição |
|---|---|---|---|
| `--create`    | `-c` | `false` | Cria um novo certificado |
| `--list`      | `-l` | `false` | Lista todos os certificados em `~/.kubediscovery/certs/` |
| `--inspect`   | `-i` | `false` | Inspeciona um certificado existente |
| `--client`    | —    | `false` | Emite certificado de **cliente** (para `kd-agent`). Se omitido, nomes que começam com `client` são auto-detectados |
| `--name`      | `-n` | `""` | Nome do certificado — **obrigatório** com `--create` |
| `--directory` | `-d` | `""` | Diretório onde o CA e os certs estão/serão salvos (padrão: `~/.kubediscovery/certs/<environment>/`) |
| `--year`      | `-y` | `3` | Validade em anos (mín 1, máx 10) |
| `--environment` | `-e` | `""` | Ambiente do servidor — **obrigatório** com `--create` |

> `--name` é **obrigatório** ao usar `--create` (validado em `PreRunE`).

---

### Fluxo de Execução

```
kdctl certificate
    │
    ├─ PreRunE: valida --create + --name obrigatório
    │
    ├─ [--list passado?]
    │       └─ serviceServer.ListAllCertificates()
    │               → glob ~/.kubediscovery/certs/*/*.crt
    │               → exibe tabela (forms.Table): environment | certificate
    │
    └─ [--create passado?]
            │
            ├─ lê: name, directory, year, environment, isClient
            │
            ├─ Auto-detecção de tipo:
            │   └─ se !isClient && name começa com "client" → isClient = true
            │
            ├─ resolve dirPath
            │   └─ vazio → ~/.kubediscovery/certs/<environment>/
            │
            ├─ sslGenerate.LoadCAFromFiles(dirPath)   ← CA existente obrigatório
            │
            ├─ monta sslGenerate.Options:
            │   ├─ herda: Organization, CommonName, Country, Locality, Province do CA
            │   ├─ Name:    <name>
            │   ├─ Years:   <year>
            │   └─ Domains: [<name>, "<name>.<env>", "localhost"]
            │
            ├─ [isClient == true?]
            │       ├─ Sim → sslGenerate.NewClientCertificate(name, opts, ca)
            │       └─ Não → sslGenerate.NewServerCertificate(name, opts, ca)
            │
            ├─ cert.WriteToFiles(dirPath)
            │
            └─ forms.Notification("Certificate created successfully!")
                    └─ exibe: Type: client|server | name | path
```

---

### Certificado de Cliente vs. Servidor

| Aspecto | Servidor (`kd-gateway`) | Cliente (`kd-agent`) |
|---|---|---|
| Flag | _(padrão)_ | `--client` ou nome prefixado `client-` |
| Função | `NewServerCertificate` | `NewClientCertificate` |
| Quem usa | `kd-gateway` para expor gRPC TLS | `kd-agent` para autenticação mTLS |
| Configurado em | `GRPC_CERT_FILE` / `GRPC_KEY_FILE` | `GRPC_CLIENT_CERT_FILE` / `GRPC_CLIENT_KEY_FILE` |
| Domains SANs | `[name, name.env, localhost]` | `[name, name.env, localhost]` |

---

### Artefatos Gerados

```
<dirPath>/                          # padrão: ~/.kubediscovery/certs/<environment>/
    ├── <name>.crt                  # Certificado TLS (servidor ou cliente)
    └── <name>.key                  # Chave privada correspondente
```

O CA (`ca.crt` / `ca.key`) deve existir previamente — criado pelo `kdctl init`.

---

### Arquivos do Módulo

| Arquivo | Responsabilidade |
|---|---|
| `cmd/certificate.go` | Define o comando `certificate`, todas as flags, `PreRunE`, `RunE`, `listAllCertificates()` e `createCertificate()` |
| `internal/service/server/certificate.go` | `ListAllCertificates()`: percorre `~/.kubediscovery/certs/*/*.crt` e retorna mapa `env → paths` |

### Bibliotecas Utilizadas

| Biblioteca | Propósito |
|---|---|
| `github.com/spf13/cobra` | Framework CLI — flags, `PreRunE`, `RunE` |
| `github.com/kubediscovery/kd-libs/sslGenerate` | `LoadCAFromFiles`, `NewServerCertificate`, `NewClientCertificate`, `WriteToFiles` |
| `github.com/kubediscovery/kd-libs/forms` | `forms.Table` (listagem) e `forms.Notification` (confirmação de sucesso) |
