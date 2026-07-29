# Restock Priority Service

Serviço HTTP que gerencia peças de reposição de autopeças e decide **quais priorizar para
reposição**, considerando estoque limitado, capital de giro limitado, padrões de venda
distintos, tempo de entrega do fornecedor e criticidade operacional.

Go 1.26 · Gin · GORM · PostgreSQL · arquitetura hexagonal

---

## Sumário

- [Começando em um comando](#começando-em-um-comando)
- [Documentação da API (Swagger)](#documentação-da-api-swagger)
- [Como rodar](#como-rodar)
- [Comandos disponíveis](#comandos-disponíveis)
- [Estrutura do projeto](#estrutura-do-projeto)
- [Como a prioridade é calculada](#como-a-prioridade-é-calculada)
- [Uma nota sobre o exemplo do enunciado](#uma-nota-sobre-o-exemplo-do-enunciado)
- [Endpoints](#endpoints)
- [Testes](#testes)
- [Documentação de engenharia](#documentação-de-engenharia)
- [Fora do escopo da v1](#fora-do-escopo-da-v1)

---

## Começando em um comando

Precisa apenas de Docker:

```bash
make docker-up
```

Isso sobe o PostgreSQL, espera ficar saudável, **roda as migrations como passo próprio** e
só então sobe a API. Quando terminar:

| O quê | Onde |
| --- | --- |
| Swagger UI | <http://localhost:8080/docs> |
| Documento OpenAPI | <http://localhost:8080/openapi.yaml> |
| API | <http://localhost:8080> |

Derrubar tudo, incluindo o volume do banco: `make docker-down`

> Se a porta 5432 ou 8080 já estiver ocupada na sua máquina:
> ```bash
> POSTGRES_PORT=5434 API_PORT=8099 docker compose up -d --build
> ```

---

## Documentação da API (Swagger)

O contrato completo é descrito em [`api/openapi.yaml`](api/openapi.yaml) (OpenAPI 3.0) e
servido pela própria aplicação:

- **`GET /docs`** — Swagger UI, navegável, com *Try it out* funcionando
- **`GET /openapi.yaml`** — o documento cru, para importar no Postman, Insomnia ou gerar client

Dois detalhes que valem saber:

**Funciona offline.** Os assets do Swagger UI são embutidos no binário e o documento é
embutido via `go:embed`. Não há CDN nem arquivo lido do disco em runtime — o container não
precisa de rede para servir a documentação.

**O documento é escrito à mão, não gerado por anotações — e existe um teste que impede ele
de mentir.** `TestOpenAPI_MatchesRegisteredRoutes` compara as rotas registradas no router
com as operações declaradas no spec e **falha nas duas direções**: rota nova sem documentar,
ou operação documentada que não é servida. Anotação em handler pode ficar obsoleta em
silêncio; isso não pode. Outros testes garantem que todo `$ref` resolve e que todo código de
erro emitido está declarado no enum (e vice-versa).

O raciocínio completo, incluindo por que não usamos `swaggo/swag`, está em
[ADR-003](docs/sdd/decisions/003-openapi-documentation.md).

---

## Como rodar

### Opção 1 — Docker (recomendado)

```bash
make docker-up      # sobe postgres -> migrations -> api
make docker-down    # derruba tudo e apaga o volume
```

A ordem é garantida pelo compose: a API declara
`depends_on: migrate: condition: service_completed_successfully`, então ela só começa a
aceitar tráfego depois das migrations terminarem.

### Opção 2 — Local

Precisa de Go 1.26 e um PostgreSQL alcançável.

```bash
cp .env.example .env    # ajuste DATABASE_URL se necessário
make migrate            # aplica as migrations versionadas
make run                # sobe a API
```

`make migrate` aplica o SQL de [`migrations/`](migrations/) com
[goose](https://github.com/pressly/goose). **A API nunca altera o schema** — migration é
passo separado e tem que rodar antes. Se o banco estiver inacessível, a API **falha ao
subir** em vez de servir e dar erro em cada request.

### Variáveis de ambiente

| Variável | Padrão | Para quê |
| --- | --- | --- |
| `PORT` | `8080` | Porta HTTP da API |
| `GIN_MODE` | `debug` | `release` em container |
| `DATABASE_URL` | montada a partir dos `DB_*` | String de conexão do PostgreSQL |
| `DB_HOST`, `DB_PORT`, `DB_USER`, `DB_PASSWORD`, `DB_NAME`, `DB_SSLMODE` | `localhost`, `5432`, `postgres`, `postgres`, `restock_priority`, `disable` | Usadas só se `DATABASE_URL` não for definida |
| `POSTGRES_PORT`, `API_PORT` | `5432`, `8080` | Portas publicadas pelo docker compose. Não afetam `make run` |

---

## Comandos disponíveis

```bash
make check            # todos os quality gates: gofmt + vet + test + test -race
make test             # testes
make test-race        # testes com detector de corrida
make cover            # cobertura de statements
make lint             # gofmt (falha se houver arquivo não formatado) + go vet
make build            # compila os dois binários em bin/

make migrate          # aplica as migrations
make migrate-status   # o que já foi aplicado
make migrate-down     # desfaz a última

make run              # sobe a API localmente
make docker-up        # sobe a stack completa
make docker-down      # derruba a stack e o volume
```

---

## Estrutura do projeto

Arquitetura hexagonal. A dependência aponta **numa direção só**: adapters dependem da
aplicação, a aplicação depende do domínio, e o domínio não depende de ninguém.

```
┌─────────────────────────────────────────────────────────────┐
│ cmd/api ...................... composition root             │
│ cmd/migrate .................. runner de migrations         │
└─────────────────────────────────────────────────────────────┘
                             │ injeta
                             ▼
┌─────────────────────────────────────────────────────────────┐
│ internal/adapter/http ........ Gin, DTOs, middleware, erros │
│ internal/adapter/postgres .... GORM, model, SQL             │
│ internal/adapter/memory ...... implementação em memória     │
└─────────────────────────────────────────────────────────────┘
                             │ dependem das portas
                             ▼
┌─────────────────────────────────────────────────────────────┐
│ internal/application ......... casos de uso + PORTAS        │
│   PartRepository · ReadinessChecker                         │
└─────────────────────────────────────────────────────────────┘
                             │ depende do domínio
                             ▼
┌─────────────────────────────────────────────────────────────┐
│ internal/domain .............. Part, invariantes,           │
│                                PriorityEngine (puro)        │
└─────────────────────────────────────────────────────────────┘
```

```
.
├── api/
│   ├── openapi.yaml            contrato OpenAPI 3.0, embutido no binário
│   └── api.go                  go:embed do documento
├── cmd/
│   ├── api/main.go             composition root: o único lugar que escolhe PostgreSQL
│   └── migrate/main.go         aplica as migrations, antes da API servir
├── internal/
│   ├── domain/
│   │   ├── part.go             entidade Part e suas invariantes
│   │   └── priority.go         PriorityEngine — as fórmulas e a ordenação, puro
│   ├── application/
│   │   ├── port.go             PartRepository e ReadinessChecker (portas)
│   │   ├── part_service.go     CRUD
│   │   └── priority_service.go busca as peças e delega o ranking ao domínio
│   ├── adapter/
│   │   ├── http/               router, handlers, DTOs, middleware, mapeamento de erros
│   │   ├── postgres/           repositório GORM, model, readiness
│   │   └── memory/             repositório em memória (usado pelos testes)
│   └── platform/config/        configuração tipada, lida uma vez no startup
├── migrations/                 SQL versionado, embutido no cmd/migrate
└── docs/                       especificação, design, ADRs, padrões de engenharia
```

### As três exigências do enunciado, e onde elas moram

**"O cálculo de prioridade deve estar isolado da camada HTTP."**
[`internal/domain/priority.go`](internal/domain/priority.go) é função pura da entrada: sem
I/O, sem relógio, sem tipo de transporte. A camada HTTP mapeia o resultado para um DTO e
não faz nenhuma aritmética.

**"A solução deve permitir futura troca de banco de dados."**
A aplicação é dona das portas `PartRepository` e `ReadinessChecker`.
[`internal/adapter/memory`](internal/adapter/memory) é uma **segunda implementação real** de
ambas — é contra ela que os testes rodam. Nenhum código de transporte segura handle de
banco. Trocar de banco é escrever um pacote irmão do `postgres`.

**"Tratar corretamente casos de estoque negativo."**
Estoque atual nunca é clampado. Valores negativos de estoque atual e projetado atravessam
as fórmulas intactos, o schema **deliberadamente** não restringe `current_stock`, e os dois
casos têm teste.

### Números

Estoque e lead time são inteiros. Vendas, custo e **todo valor calculado** usam decimal
exato (`github.com/shopspring/decimal`). `float64` nunca representa dinheiro nem participa
de comparação de desempate — então igualdade no ranking é exata, e um `urgencyScore` maior
que `int64` continua correto em vez de estourar.

### Escala

O ranking custa `O(n)` para calcular e `O(n log n)` para ordenar, em processo, com memória
`O(n)`. Para os milhares de peças do escopo isso cabe folgado em um request. Ranking em
SQL, score pré-calculado e cache foram **deliberadamente adiados** até uma medição dizer o
contrário.

---

## Como a prioridade é calculada

```
expectedConsumption = averageDailySales × leadTimeDays
projectedStock      = currentStock − expectedConsumption
precisa de reposição quando   projectedStock < minimumStock
urgencyScore        = (minimumStock − projectedStock) × criticalityLevel
```

A comparação de elegibilidade é **estrita**: peça cujo estoque projetado é *igual* ao
mínimo fica fora.

Ordenação, em cascata:

| # | Critério | Origem |
| --- | --- | --- |
| 1 | `urgencyScore` desc | enunciado |
| 2 | `criticalityLevel` desc | enunciado |
| 3 | `averageDailySales` desc | enunciado |
| 4 | `name` asc, case-insensitive | enunciado |
| 5 | identificador asc | **nosso** — garante ordem total |

O nível 5 vai além do que o enunciado pede, de propósito: sem ele, duas peças idênticas em
tudo poderiam sair em ordem diferente entre requests. Com ele, **o mesmo dado sempre produz
a mesma sequência**.

`unitCost` é armazenado e devolvido, mas não afeta elegibilidade nem prioridade.

---

## Uma nota sobre o exemplo do enunciado

O exemplo de resposta do enunciado é **inconsistente com as fórmulas que o próprio
enunciado define**. Para `currentStock: 15`, `averageDailySales: 4`, `leadTimeDays: 5`,
`minimumStock: 20`, `criticalityLevel: 3`:

```
expectedConsumption = 4 × 5         = 20
projectedStock      = 15 − 20       = −5     o exemplo mostra 5
urgencyScore        = (20 − −5) × 3 = 75     o exemplo mostra 45
```

A **segunda** linha do exemplo — Pastilha de Freio Y, `8 / −2 / 10 / 36` — *é* consistente
com as mesmas fórmulas. É isso que confirma que a divergência está na primeira linha, e não
nas fórmulas.

**Este serviço implementa as fórmulas**, então devolve `−5` e `75`. As regras escritas são
tratadas como autoritativas (BR-014), a divergência está registrada em
[docs/project-overview.md](docs/project-overview.md), e copiar o exemplo inconsistente para
dentro de um teste é explicitamente proibido em
[docs/engineering/anti-patterns.md](docs/engineering/anti-patterns.md).

---

## Endpoints

Campos em camelCase. Decimais são **números** JSON, nunca string entre aspas.

| Método | Caminho | O quê |
| --- | --- | --- |
| `POST` | `/parts` | Cria peça. `201` com header `Location` |
| `GET` | `/parts` | Lista peças. `category`, `limit` (1–100, padrão 50), `offset` |
| `GET` | `/parts/{id}` | Busca uma peça |
| `PUT` | `/parts/{id}` | Substitui todos os campos mutáveis. Update parcial não existe |
| `DELETE` | `/parts/{id}` | Remove peça. `204` sem corpo |
| `GET` | `/restock/priorities` | Peças a repor, ranqueadas. Sem query params na v1 |
| `GET` | `/healthz` | Liveness. Não toca no PostgreSQL |
| `GET` | `/readyz` | Readiness. Checagem limitada no banco; `503` em falha ou timeout |
| `GET` | `/docs` | Swagger UI |
| `GET` | `/openapi.yaml` | Documento OpenAPI |

Erros sempre usam o mesmo envelope, com código estável e legível por máquina:

```json
{
  "error": {
    "code": "validation_error",
    "message": "the request contains invalid fields",
    "fields": { "criticalityLevel": "must be between 1 and 5" }
  }
}
```

Códigos: `invalid_request`, `validation_error`, `part_not_found`, `not_found`,
`method_not_allowed`, `request_too_large`, `internal_error`, `service_unavailable`.

### Exemplo

```bash
curl -X POST localhost:8080/parts -H 'Content-Type: application/json' -d '{"name":"Filtro de Óleo X","category":"engine","currentStock":15,"minimumStock":20,"averageDailySales":4,"leadTimeDays":5,"unitCost":18.50,"criticalityLevel":3}'
```

```bash
curl localhost:8080/restock/priorities
```

```json
{
  "priorities": [
    {
      "partId": "542fbeeb-8382-4d1e-97d1-00c67a6486e7",
      "name": "Filtro de Óleo X",
      "currentStock": 15,
      "projectedStock": -5,
      "minimumStock": 20,
      "urgencyScore": 75
    }
  ]
}
```

O contrato completo — todo schema, todo status, todo header — está no
[Swagger UI](http://localhost:8080/docs) e em
[SPEC-001](docs/sdd/specs/001-core-service.md).

---

## Testes

```bash
make check
```

Roda os quality gates de
[docs/engineering/go-guidelines.md](docs/engineering/go-guidelines.md): verificação de
`gofmt`, `go vet`, a suíte, e a suíte de novo com detector de corrida.

Os testes são table-driven e nomeados pelo critério de aceite que verificam. Os cenários
extremos que o enunciado cobra têm cobertura explícita: **estoque negativo**, **venda
zero**, **lead time altíssimo** (`math.MaxInt32`), lead time zero, estoque projetado
exatamente igual ao mínimo, vendas fracionárias, e um `urgencyScore` de
`46116860184273879035` — que estouraria `int64` mas sai exato em decimal.

`make cover` reporta cobertura de statements — com `-coverpkg`, senão o adapter de memória
aparece como 0% só porque quem o exercita são os testes de outro pacote.

| Pacote | Cobertura |
| --- | --- |
| `internal/domain` | 100% |
| `internal/application` | 100% |
| `internal/adapter/http` | ~99% |
| `internal/adapter/memory` | ~97% |
| `internal/adapter/postgres` | ~39% |
| `cmd/*`, `internal/platform/config` | sem teste |

O `postgres` é parcial de propósito: os testes dele verificam **o SQL que o adapter monta**,
usando o `DryRun` do GORM — sem precisar de banco — e é isso que trava a regressão do `PUT`
que descartava zeros. O comportamento que só um PostgreSQL de verdade demonstra (constraints,
timestamps, migrations em ordem) está coberto por verificação manual registrada em
[SPEC-001](docs/sdd/specs/001-core-service.md). Teste de integração automatizado contra
Postgres real continua sendo a lacuna conhecida.

---

## Documentação de engenharia

| Assunto | Documento |
| --- | --- |
| Índice | [docs/README.md](docs/README.md) |
| Comportamento, contrato e critérios de aceite (normativo) | [SPEC-001](docs/sdd/specs/001-core-service.md) |
| Arquitetura e direção de dependência | [system design](docs/architecture/system-design.md) |
| Padrões de Go | [go-guidelines](docs/engineering/go-guidelines.md) |
| O que é proibido e por quê | [anti-patterns](docs/engineering/anti-patterns.md) |
| Decisões duráveis | [ADR-001 (Gin)](docs/sdd/decisions/001-http-framework-gin.md) · [ADR-002 (GORM)](docs/sdd/decisions/002-persistence-gorm.md) |
| Processo de spec | [SDD workflow](docs/sdd/README.md) |

---

## Fora do escopo da v1

Autenticação, gestão de fornecedores, ordens de compra, quantidade recomendada de compra,
alocação de orçamento, cache, filas e jobs em background. `unitCost` é armazenado e
devolvido, mas não entra no ranking (BR-013). A lista completa, com as razões, está em
[docs/project-overview.md](docs/project-overview.md).

Adiado da entrega, não do escopo: testes de integração automatizados contra PostgreSQL,
teste end-to-end automatizado, workflow de CI, limites de pool de conexão e timeouts
configuráveis validados.
