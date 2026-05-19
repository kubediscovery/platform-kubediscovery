# 0006. kd-mcp como Componente Dedicado para o Protocolo MCP

- Status: accepted
- Date: 2026-05-19

## Context

A plataforma precisa expor o protocolo MCP (Model Context Protocol) para clientes LLM externos como Claude Desktop, Cursor e IDEs. O `kd-gateway` já é o ponto focal de orquestração interna via gRPC. Misturar o protocolo MCP no gateway acoplaria um protocolo externo em evolução ao core interno da plataforma.

## Considered Options

- **`kd-mcp` como componente dedicado:** separação de responsabilidades clara; MCP evolui independentemente do gateway; pode ser escalado horizontalmente de forma independente; stateless por design.
- **MCP embutido no `kd-gateway`:** um componente a menos para operar; mas acopla protocolo externo ao core; mudanças no protocolo MCP impactam o gateway; dificulta escalar MCP independentemente do gateway.

## Decision

`kd-mcp` é um componente Go dedicado que expõe o protocolo MCP para clientes externos e traduz tool calls para chamadas gRPC ao `kd-gateway`. É stateless — todo estado reside no gateway e no `kd-store`.

## Consequences

- **Positivo:** `kd-gateway` permanece focado em orquestração interna sem acoplamento a protocolos externos.
- **Positivo:** `kd-mcp` pode ser escalado horizontalmente via HPA independentemente do gateway.
- **Positivo:** Atualizações do protocolo MCP (novas versões, novos clientes) não afetam o gateway.
- **Negativo:** Um componente adicional para operar, monitorar e fazer deploy.
- **Negativo:** Latência adicional de um hop gRPC entre `kd-mcp` e `kd-gateway`.
- **Follow-up:** Definir contrato gRPC entre `kd-mcp` e `kd-gateway` no `proto/kubediscovery/v1/mcp.proto`.
