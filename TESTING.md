# Testes e Coverage

## Pré-requisitos

- Go 1.25+
- Docker (para testcontainers)

## Como funciona

Os testes de aceitação usam [testcontainers-go](https://github.com/testcontainers/testcontainers-go) para subir um container PostgreSQL 16 automaticamente. Nenhuma configuração manual de banco é necessária.

O `TestMain` em `internal/provider/provider_test.go`:

1. Sobe um container `postgres:16-alpine` com `max_connections=500`
2. Aguarda o banco ficar pronto (log `"database system is ready to accept connections"`)
3. Descobre host e porta mapeados pelo Docker
4. Seta as variáveis de ambiente `PGHOST`, `PGPORT`, `PGUSER`, `PGPASSWORD`, `PGDATABASE`, `PGSSLMODE`
5. Executa todos os testes
6. Destrói o container ao finalizar

Se `PGHOST` já estiver definido no ambiente, o container **não** é criado e os testes usam o banco externo apontado pelas variáveis PG*.

## Executando os testes

### Todos os testes (com testcontainer)

```bash
TF_ACC=1 go test ./internal/provider/ -v -parallel 1 -timeout 600s
```

> **`-parallel 1`** é recomendado para evitar exaustão de conexões. Cada test step do Terraform cria um processo separado com sua própria pool de conexões.

### Usando o Makefile

```bash
make testacc
```

### Teste específico

```bash
TF_ACC=1 go test ./internal/provider/ -v -parallel 1 -run "TestAccPostgresqlRole_basic"
```

### Com banco externo (sem Docker)

```bash
export PGHOST=localhost
export PGPORT=5432
export PGUSER=postgres
export PGPASSWORD=minha_senha
export PGDATABASE=postgres
export PGSSLMODE=disable

TF_ACC=1 go test ./internal/provider/ -v -parallel 1 -timeout 600s
```

## Coverage

### Gerar relatório de coverage

```bash
TF_ACC=1 go test ./internal/provider/ -parallel 1 -timeout 600s -coverprofile=coverage.out
```

### Visualizar coverage por função

```bash
go tool cover -func=coverage.out
```

Saída exemplo:

```
github.com/.../provider.go:85:     Configure    80.0%
github.com/.../role_resource.go:137:   Create       74.1%
...
total:                             (statements) 80.8%
```

### Gerar relatório HTML

```bash
go tool cover -html=coverage.out -o coverage.html
```

Abra o arquivo `coverage.html` no navegador para ver linhas cobertas (verde) e não cobertas (vermelho).

### Funções com menor coverage e por quê

| Função | Coverage | Motivo |
|---|---|---|
| `Configure` (todos) | 71.4% | Branch de erro quando `ProviderData` tem tipo errado - não testável em acceptance tests |
| `database_resource.Read` | 46.2% | Path de "database not found" requer deletar o banco fora do Terraform durante o teste |
| `envOrDefault` | 66.7% | Branch de variável de ambiente definida - coberto implicitamente pelo testcontainer |

## Estrutura dos testes

```
internal/provider/
├── provider_test.go                    # TestMain (testcontainer) + helpers
├── role_resource_test.go               # 8 testes
├── role_data_source_test.go            # 4 testes
├── database_resource_test.go           # 6 testes
├── database_data_source_test.go        # 3 testes
├── schema_resource_test.go             # 5 testes
├── schemas_data_source_test.go         # 6 testes
├── grant_resource_test.go              # 13 testes
├── default_privileges_resource_test.go # 7 testes
└── query_data_source_test.go           # 6 testes
```

**Total: 58 testes de aceitação**

## Variáveis de ambiente

| Variável | Default (testcontainer) | Descrição |
|---|---|---|
| `TF_ACC` | - | Obrigatório para rodar testes de aceitação |
| `PGHOST` | (auto) | Host do PostgreSQL. Se definido, pula o testcontainer |
| `PGPORT` | (auto) | Porta do PostgreSQL |
| `PGUSER` | `postgres` | Usuário |
| `PGPASSWORD` | `postgres` | Senha |
| `PGDATABASE` | `postgres` | Banco padrão |
| `PGSSLMODE` | `disable` | Modo SSL |

## Troubleshooting

### `pq: sorry, too many clients already`

O PostgreSQL atingiu o limite de conexões. Soluções:

- Use `-parallel 1` ao rodar os testes
- O container já sobe com `max_connections=500`, mas cada test step cria conexões que demoram para fechar
- O provider configura `MaxOpenConns(5)` por instância

### Container não inicia

Verifique se o Docker está rodando:

```bash
docker info
```

### Testes lentos

Todos os 58 testes levam ~30 segundos com `-parallel 1`. O overhead é do startup do container (~2s) e do binário do Terraform em cada test step.
