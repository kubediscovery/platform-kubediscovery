## ADDED Requirements

### Requirement: Servidor MCP dedicado para clientes LLM externos

Feature: kd-mcp expõe o protocolo MCP para clientes externos como Claude Desktop, Cursor e IDEs
Rule: kd-mcp é um componente separado do kd-gateway — traduz MCP para gRPC sem misturar responsabilidades.

#### Scenario: Cliente LLM conecta via MCP e executa tool call
- **GIVEN** Claude Desktop configurado com endpoint do kd-mcp
- **WHEN** o usuário solicita "liste os pods do namespace production no cluster prod-us-east"
- **THEN** kd-mcp traduz para chamada gRPC ao kd-gateway, executa e retorna resultado formatado ao cliente MCP

#### Scenario: kd-mcp autentica no kd-gateway via gRPC
- **GIVEN** kd-mcp com certificado de serviço válido
- **WHEN** estabelece conexão com o kd-gateway
- **THEN** conexão é aceita e kd-mcp pode rotear comandos para agentes registrados

#### Scenario: Tool call para agente desconectado
- **GIVEN** cliente MCP solicitando operação em cluster `cluster-offline`
- **WHEN** kd-mcp roteia para o gateway
- **THEN** retorna erro MCP estruturado: `{"error": "agent cluster-offline is not connected"}`

### Requirement: Catálogo de tools MCP mapeadas para operações da plataforma

Feature: kd-mcp expõe tools MCP correspondentes às operações disponíveis na plataforma
Rule: Cada tool MCP deve mapear para uma operação gRPC específica no kd-gateway.

#### Scenario: Tool list_pods disponível no catálogo MCP
- **GIVEN** cliente MCP consultando tools disponíveis
- **WHEN** realiza `tools/list`
- **THEN** recebe catálogo incluindo `list_pods`, `get_pod_logs`, `list_deployments`, `analyze_cluster` entre outros

#### Scenario: Tool com parâmetros obrigatórios ausentes
- **GIVEN** chamada MCP para `list_pods` sem parâmetro `cluster_id`
- **WHEN** kd-mcp valida os parâmetros
- **THEN** retorna erro MCP `INVALID_PARAMS` com campo ausente identificado

### Requirement: Escalabilidade stateless do kd-mcp

Feature: kd-mcp é stateless e pode escalar horizontalmente
Rule: Nenhum estado de sessão é mantido no kd-mcp — todo estado reside no kd-gateway e kd-store.

#### Scenario: Múltiplas réplicas do kd-mcp atendem clientes simultaneamente
- **GIVEN** 3 réplicas do kd-mcp rodando atrás de um load balancer
- **WHEN** 3 clientes MCP diferentes enviam tool calls simultaneamente
- **THEN** cada réplica processa sua chamada independentemente sem conflito de estado

## MODIFIED Requirements

## REMOVED Requirements
