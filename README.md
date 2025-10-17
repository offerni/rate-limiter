# Rate Limiter / Limitador de Taxa

Middleware de limitação de taxa HTTP para aplicações Go usando roteador Chi.

HTTP rate limiting middleware for Go applications using Chi router.

---

## Português

### Pré-requisitos

- Docker e Docker Compose instalados
- curl e jq para testes (opcional)

### Início Rápido

1. **Construir e iniciar o container:**

   ```bash
   docker-compose up --build -d
   ```

2. **Testar funcionalidade básica:**
   ```bash
   curl http://localhost:8080/
   curl http://localhost:8080/health
   ```

### Testando Limitação de Taxa

#### Limitação por IP (10 requisições/segundo)

```bash
# Fazer 15 requisições rápidas para ativar o limite
for i in {1..15}; do
  echo "Requisição $i:"
  curl -s http://localhost:8080/ | jq .
done
```

Esperado: Primeiras 10 requisições passam, depois erros 429.

#### Limitação por Token (100 requisições/segundo)

```bash
# Testar com chave API
for i in {1..15}; do
  echo "Requisição $i com chave API:"
  curl -s -H "API_KEY: ABC123" http://localhost:8080/api/test | jq .
done
```

Esperado: Todas as 15 requisições passam.

#### Diferentes Limites de Token

```bash
# Testar diferentes tokens
curl -H "API_KEY: XYZ789" http://localhost:8080/api/test  # Limite de 50/segundo
curl -H "API_KEY: UNKNOWN" http://localhost:8080/api/test  # Volta para limite de IP
```

#### Teste de Carga

```bash
# Testar endpoint de carga (mostra IP do cliente)
curl http://localhost:8080/api/load-test

# Teste de carga com chave API
curl -H "API_KEY: ABC123" http://localhost:8080/api/load-test
```

### Endpoints Disponíveis

- `GET /` - Endpoint básico
- `GET /health` - Verificação de saúde
- `GET /api/test` - Teste de chave API
- `GET /api/load-test` - Endpoint de teste de carga

### Configuração

Edite o arquivo `.env` para personalizar limites:

```env
IP_RATE_LIMIT=10
IP_BLOCK_TIME=300
TOKEN_ABC123_LIMIT=100
TOKEN_ABC123_BLOCK_TIME=300
SERVER_PORT=8080
```

### Monitoramento

```bash
# Ver logs
docker-compose logs -f

# Parar container
docker-compose down
```

---

## English

### Prerequisites

- Docker and Docker Compose installed
- curl and jq for testing (optional)

### Quick Start

1. **Build and start the container:**

   ```bash
   docker-compose up --build -d
   ```

2. **Test basic functionality:**
   ```bash
   curl http://localhost:8080/
   curl http://localhost:8080/health
   ```

### Testing Rate Limiting

#### IP-based Rate Limiting (10 requests/second)

```bash
# Make 15 rapid requests to trigger rate limit
for i in {1..15}; do
  echo "Request $i:"
  curl -s http://localhost:8080/ | jq .
done
```

Expected: First 10 requests succeed, then 429 errors.

#### Token-based Rate Limiting (100 requests/second)

```bash
# Test with API key
for i in {1..15}; do
  echo "Request $i with API key:"
  curl -s -H "API_KEY: ABC123" http://localhost:8080/api/test | jq .
done
```

Expected: All 15 requests succeed.

#### Different Token Limits

```bash
# Test different tokens
curl -H "API_KEY: XYZ789" http://localhost:8080/api/test  # 50/second limit
curl -H "API_KEY: UNKNOWN" http://localhost:8080/api/test  # Falls back to IP limit
```

#### Load Testing

```bash
# Test load endpoint (shows client IP)
curl http://localhost:8080/api/load-test

# Load test with API key
curl -H "API_KEY: ABC123" http://localhost:8080/api/load-test
```

### Available Endpoints

- `GET /` - Basic endpoint
- `GET /health` - Health check
- `GET /api/test` - API key testing
- `GET /api/load-test` - Load testing endpoint

### Configuration

Edit `.env` file to customize limits:

```env
IP_RATE_LIMIT=10
IP_BLOCK_TIME=300
TOKEN_ABC123_LIMIT=100
TOKEN_ABC123_BLOCK_TIME=300
SERVER_PORT=8080
```

### Monitoring

```bash
# View logs
docker-compose logs -f

# Stop container
docker-compose down
```

---

## Resposta de Erro / Error Response

Quando limite excedido (HTTP 429) / When rate limit exceeded (HTTP 429):

```json
{
  "error": "you have reached the maximum number of requests or actions allowed within a certain time frame"
}
```
