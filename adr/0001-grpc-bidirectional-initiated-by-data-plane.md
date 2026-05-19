# 0001. gRPC Bidirecional Iniciado pelo Data Plane

- Status: accepted
- Date: 2026-05-19

## Context

Clusters remotos (Data Plane) frequentemente operam em redes privadas sem IP público exposto. Exigir que o Control Plane inicie conexões para os agentes tornaria o deployment complexo — VPNs, regras de firewall e IPs estáticos seriam necessários em cada cluster remoto. A plataforma precisa de comunicação bidirecional em tempo real entre Control Plane e N clusters remotos com latência baixa e resiliência a falhas de rede.

## Considered Options

- **gRPC bidirecional iniciado pelo Data Plane (kd-agent faz outbound):** apenas o gateway precisa de endpoint público; agentes em redes privadas funcionam sem configuração de firewall inbound.
- **gRPC iniciado pelo Control Plane (gateway conecta nos agentes):** requer IP público ou VPN em cada cluster remoto; operacionalmente complexo em ambientes multi-cloud e on-premise.
- **Service mesh (Istio/Linkerd) com mTLS gerenciado:** abstrai a conectividade mas adiciona dependência de infraestrutura específica e complexidade operacional significativa.

## Decision

O `kd-agent` (Data Plane) inicia a conexão gRPC de saída para o `kd-gateway` (Control Plane), estabelecendo um stream bidirecional persistente. O gateway é o único componente que requer endpoint público.

## Consequences

- **Positivo:** Clusters remotos em redes privadas funcionam sem configuração de firewall inbound; deployment simplificado.
- **Positivo:** Modelo de segurança claro — o Control Plane nunca inicia conexões para o Data Plane.
- **Negativo:** O `kd-agent` deve implementar lógica de reconexão com backoff exponencial para lidar com falhas de rede.
- **Negativo:** O gateway precisa de mecanismo de heartbeat para detectar agentes desconectados que não fecharam o stream graciosamente.
- **Follow-up:** Definir política de timeout e heartbeat interval no protocolo gRPC do gateway.
