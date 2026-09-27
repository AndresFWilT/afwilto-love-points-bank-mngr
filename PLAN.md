# Plan del backend — Go, hexagonal

## 1. La regla que manda

**Los puertos hablan el idioma del negocio. Los adaptadores hablan el idioma de la
tecnología.**

En la práctica:

- Un puerto nunca menciona `*sql.Rows`, `*http.Request`, `pgx`, `json`, ni un DTO.
  Solo tipos del dominio y errores del dominio.
- Un adaptador nunca contiene una regla de negocio. Si un `if` de un handler decide algo
  sobre puntitos, está en el lugar equivocado.
- La flecha de dependencia apunta siempre hacia adentro: adaptador → aplicación →
  dominio. `internal/domain` no importa absolutamente nada del proyecto.

Prueba de humo: si mañana cambiamos Postgres por SQLite, o HTTP por una CLI, solo se
tocan carpetas bajo `internal/adapter/`. Si hay que tocar `internal/domain`, el diseño
está mal.

## 2. Estructura

```
backend/
├── cmd/api/main.go                    # composition root: el único que conoce a todos
├── internal/
│   ├── domain/                        # el hexágono. Cero dependencias externas.
│   │   ├── member.go
│   │   ├── entry.go                   # PointEntry + invariantes
│   │   ├── ledger.go                  # cálculo de saldos
│   │   └── errors.go                  # errores tipados del dominio
│   ├── app/
│   │   ├── port/
│   │   │   ├── in/                    # puertos de entrada (driving)
│   │   │   │   └── points.go
│   │   │   └── out/                   # puertos de salida (driven)
│   │   │       ├── repository.go
│   │   │       └── system.go          # Clock, IDGenerator
│   │   ├── service/
│   │   │   └── points_service.go      # orquestación, transacciones, nada de reglas
│   │   └── testsupport/
│   │       ├── memory_repo.go         # adaptador en memoria, escrito a mano
│   │       └── fixed_clock.go
│   └── adapter/
│       ├── in/http/                   # chi, DTOs, mapeo de errores → status
│       │   ├── router.go
│       │   ├── points_handler.go
│       │   ├── dto.go
│       │   └── problem.go
│       └── out/postgres/              # pgx, SQL, mapeo fila → entidad
│           ├── entry_repository.go
│           ├── member_repository.go
│           └── tx.go
├── Dockerfile
└── go.mod
```

## 3. Dominio

```go
// internal/domain/entry.go
package domain

type EntryStatus string

const (
    StatusActive EntryStatus = "active"
    StatusVoided EntryStatus = "voided"
)

const (
    MinAmount     = 1
    MaxAmount     = 5
    MinReasonRune = 10
)

type PointEntry struct {
    ID         EntryID
    AwardedTo  MemberID
    AwardedBy  MemberID
    Amount     int
    Reason     string
    OccurredAt time.Time
    Status     EntryStatus
    CreatedAt  time.Time
    UpdatedAt  time.Time
}

// NewPointEntry es el único camino para crear una entrada válida.
func NewPointEntry(id EntryID, to, by MemberID, amount int, reason string,
    occurredAt, now time.Time) (PointEntry, error)

// Edit devuelve una copia modificada y la revisión que documenta el cambio.
func (e PointEntry) Edit(amount int, reason string, occurredAt, now time.Time,
    editedBy MemberID) (PointEntry, Revision, error)

func (e PointEntry) Void(now time.Time, by MemberID) (PointEntry, error)
```

### Invariantes (van una a una a `entry_test.go`)

| ID | Regla |
|---|---|
| RD-1 | `Amount` entre 1 y 5 inclusive. Nunca cero ni negativo. |
| RD-2 | `Reason` con al menos 10 caracteres útiles tras recortar espacios. |
| RD-3 | `AwardedBy != AwardedTo`. Nadie se abona a sí mismo. |
| RD-4 | `OccurredAt` no puede ser futuro respecto a `now`. |
| RD-5 | Solo `AwardedBy` puede editar o anular la entrada. |
| RD-6 | Una entrada anulada no se edita ni se vuelve a anular. |
| RD-7 | Toda edición produce una `Revision` con los valores anteriores. |
| RD-8 | El saldo de un titular es la suma de entradas activas a su favor, menos canjes. |

El error de cada invariante es un valor exportado (`ErrReasonTooShort`,
`ErrSelfAward`, `ErrNotTheAwarder`, ...) para que el adaptador HTTP los mapee con
`errors.Is` sin comparar strings.

## 4. Puertos

```go
// internal/app/port/in/points.go — lo que el mundo exterior le puede pedir a la app
package in

type AwardCommand struct {
    AwardedTo  domain.MemberID
    AwardedBy  domain.MemberID
    Amount     int
    Reason     string
    OccurredAt time.Time
}

type PointsUseCase interface {
    Award(ctx context.Context, cmd AwardCommand) (domain.PointEntry, error)
    Edit(ctx context.Context, cmd EditCommand) (domain.PointEntry, error)
    Void(ctx context.Context, id domain.EntryID, by domain.MemberID) error
    Ledger(ctx context.Context, f LedgerFilter) ([]domain.PointEntry, error)
    Balances(ctx context.Context) ([]domain.Balance, error)
}
```

```go
// internal/app/port/out/repository.go — lo que la app necesita del mundo exterior
package out

type EntryRepository interface {
    Save(ctx context.Context, e domain.PointEntry) error
    Update(ctx context.Context, e domain.PointEntry, r domain.Revision) error
    FindByID(ctx context.Context, id domain.EntryID) (domain.PointEntry, error)
    List(ctx context.Context, f in.LedgerFilter) ([]domain.PointEntry, error)
    Balances(ctx context.Context) ([]domain.Balance, error)
}

type MemberRepository interface {
    Exists(ctx context.Context, id domain.MemberID) (bool, error)
    All(ctx context.Context) ([]domain.Member, error)
}

type TxManager interface {
    WithinTx(ctx context.Context, fn func(ctx context.Context) error) error
}
```

```go
// internal/app/port/out/system.go
type Clock interface{ Now() time.Time }
type IDGenerator interface{ NewEntryID() domain.EntryID }
```

Fíjate que ni un solo tipo de `database/sql`, `pgx` o `net/http` aparece arriba. Eso es
lo que hace que los tests de aplicación no necesiten Docker.

## 5. Servicio de aplicación

Orquesta: pide datos, llama al dominio, persiste. No decide reglas.

```go
func (s *PointsService) Award(ctx context.Context, cmd in.AwardCommand) (domain.PointEntry, error) {
    ok, err := s.members.Exists(ctx, cmd.AwardedTo)
    if err != nil { return domain.PointEntry{}, err }
    if !ok { return domain.PointEntry{}, domain.ErrMemberNotFound }

    entry, err := domain.NewPointEntry(          // <- aquí viven las reglas
        s.ids.NewEntryID(), cmd.AwardedTo, cmd.AwardedBy,
        cmd.Amount, cmd.Reason, cmd.OccurredAt, s.clock.Now(),
    )
    if err != nil { return domain.PointEntry{}, err }

    if err := s.entries.Save(ctx, entry); err != nil {
        return domain.PointEntry{}, err
    }
    return entry, nil
}
```

## 6. Adaptadores

### 6.1 HTTP (entrada)

`chi` para rutas, `net/http` para lo demás. Sin framework pesado.

| Método | Ruta | operationId | Spec |
|---|---|---|---|
| POST | `/api/v1/entries` | `awardPoint` | 001 |
| GET | `/api/v1/entries` | `listLedger` | 002 |
| PATCH | `/api/v1/entries/{id}` | `editEntry` | 003 |
| POST | `/api/v1/entries/{id}/void` | `voidEntry` | 004 |
| GET | `/api/v1/balances` | `getBalances` | 005 |
| GET | `/api/v1/members` | `listMembers` | — |
| GET | `/healthz` | — | — |

Errores en formato **RFC 7807** (`application/problem+json`). El mapeo error de dominio →
status vive solo aquí:

```go
var statusByError = map[error]int{
    domain.ErrReasonTooShort: http.StatusUnprocessableEntity,
    domain.ErrAmountOutOfRange: http.StatusUnprocessableEntity,
    domain.ErrSelfAward:      http.StatusUnprocessableEntity,
    domain.ErrFutureDate:     http.StatusUnprocessableEntity,
    domain.ErrNotTheAwarder:  http.StatusForbidden,
    domain.ErrEntryVoided:    http.StatusConflict,
    domain.ErrEntryNotFound:  http.StatusNotFound,
}
```

**Identidad en v1:** no hay login. Un header `X-Member-Id` indica quién está usando la
app, y el frontend lo fija con un selector de titular. Está documentado como deuda
consciente en `docs/specs/000-identidad.md`. Cuando llegue autenticación real, solo
cambia el middleware del adaptador.

### 6.2 Postgres (salida)

`pgx/v5` con pool. Sin ORM: el SQL se escribe a mano y se lee.

- Cada método del repositorio hace exactamente una consulta o una transacción.
- El mapeo fila → entidad ocurre en una función privada `scanEntry`.
- La entidad se reconstruye con un constructor `domain.RehydratePointEntry(...)` que
  **no revalida** las invariantes: los datos ya guardados son válidos por definición, y
  revalidarlos haría que un cambio de regla rompiera la lectura del histórico.
- `TxManager` guarda la transacción en el `context`; `Update` escribe la entrada y su
  revisión en la misma transacción.

## 7. Plan de pruebas

```
internal/domain/entry_test.go          # RD-1..RD-7, tabla, sin mocks
internal/domain/ledger_test.go         # RD-8
internal/app/service/points_test.go    # CA-x con testsupport.MemoryRepo
internal/adapter/out/postgres/*_test.go  //go:build integration  (testcontainers)
internal/adapter/in/http/*_test.go     # httptest + fake PointsUseCase
test/e2e/smoke_test.go                 //go:build e2e
```

Ejemplo del arranque en rojo de RD-2:

```go
func TestNewPointEntry_RejectsShortReason(t *testing.T) {
    cases := []struct{ name, reason string }{
        {"vacía", ""},
        {"solo espacios", "      "},
        {"muy corta", "por linda"},
    }
    for _, tc := range cases {
        t.Run(tc.name, func(t *testing.T) {
            _, err := domain.NewPointEntry(id, to, by, 3, tc.reason, ayer, ahora)
            if !errors.Is(err, domain.ErrReasonTooShort) {
                t.Fatalf("esperaba ErrReasonTooShort, obtuve %v", err)
            }
        })
    }
}
```

Integración con testcontainers: un solo contenedor por paquete (`TestMain`), migraciones
aplicadas al arrancar, y `TRUNCATE ... RESTART IDENTITY CASCADE` entre tests. Nada de
compartir estado entre casos.

## 8. Dockerfile

```dockerfile
FROM golang:1.23-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/api ./cmd/api

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/api /api
USER nonroot:nonroot
EXPOSE 8080
ENTRYPOINT ["/api"]
```

Configuración por variables de entorno, leídas una sola vez en `main.go`:
`DATABASE_URL`, `HTTP_ADDR`, `LOG_LEVEL`, `CORS_ORIGIN`.

Apagado ordenado: `signal.NotifyContext` + `server.Shutdown` con 10s de gracia, y cierre
del pool al final.

## 9. Orden de implementación

1. `go mod init`, Dockerfile, `/healthz`, compose arriba. Nada de dominio todavía.
2. Spec 001 → tests de dominio → `PointEntry` → puerto → `MemoryRepo` → servicio →
   handler → repositorio Postgres. Vertical completa, una sola funcionalidad.
3. Spec 002 (listado con filtros y paginación por cursor).
4. Spec 005 (saldos, vista materializada no; vista simple sí).
5. Spec 003 y 004 (edición y anulación, con revisiones).
6. E2E de humo y pulido.

## 10. Fase 2 — catálogo y canjes (specs 006, 007)

Misma disciplina hexagonal. El catálogo y los canjes son un dominio nuevo que cuelga de
`members`; nada de lo de fase 1 se toca salvo la vista de saldos.

### 10.1 Dominio

```go
// internal/domain/catalog.go
type Tier string
const (
    TierBaratito Tier = "baratito"
    TierMedio    Tier = "medio"
    TierAlto     Tier = "alto"
)

const (
    MinCost        = 1
    TierMedioFrom  = 6   // ≤5 baratito, 6..50 medio, ≥51 alto
    TierAltoFrom   = 51
    MinNameRune    = 3
)

// TierFor es la ÚNICA fuente de la derivación en el código; la base la replica en la
// columna generada. Si cambia el umbral, cambia aquí y en la migración (nueva migración).
func TierFor(cost int) Tier

type CatalogItem struct {
    ID          ItemID
    Name        string
    Description string
    Cost        int
    IsActive    bool
    // Tier se deriva de Cost; no se persiste como entrada, se lee de la columna generada.
    CreatedAt, UpdatedAt time.Time
}
func NewCatalogItem(id ItemID, name, desc string, cost int, now time.Time) (CatalogItem, error)
func (c CatalogItem) Edit(name, desc string, cost int, isActive bool, now time.Time) (CatalogItem, error)

// internal/domain/redemption.go
type Redemption struct {
    ID            RedemptionID
    MemberID      MemberID
    CatalogItemID ItemID
    Cost          int          // cost_at_redemption, congelado
    Note          string
    Status        EntryStatus  // reutiliza active/voided
    RedeemedAt, UpdatedAt time.Time
}

// NewRedemption valida RD-CAN-1/3/5 y congela el costo. La regla de saldo (RD-CAN-2) NO
// va aquí: necesita el disponible, que el servicio calcula dentro de la transacción.
func NewRedemption(id RedemptionID, by MemberID, item CatalogItem, amount int,
    note string, now time.Time) (Redemption, error)
func (r Redemption) Void(now time.Time, by MemberID) (Redemption, error)
```

Invariantes nuevas (una a una a los tests de dominio):

| ID | Regla |
|---|---|
| RD-CAT-1 | `Cost` ≥ 1. |
| RD-CAT-2 | `Name` con ≥ 3 caracteres útiles. |
| RD-CAT-3 | `Tier` derivado de `Cost` por `TierFor`. |
| RD-CAN-1 | `amount` del canje ≥ 1. |
| RD-CAN-3 | el plan debe estar activo al canjear. |
| RD-CAN-4 | `Cost` del canje = `amount` congelado. |
| RD-CAN-6 | solo quien canjeó puede anular. |
| RD-CAN-7 | un canje anulado no se re-anula. |
| RD-CAN-9 | `note` con ≥ 10 caracteres útiles (el porqué es obligatorio). |

RD-CAN-2 (sin sobregiro) y RD-CAN-8 (saldo propio) son reglas de **aplicación**: viven en
el servicio, dentro de la transacción, no en la entidad.

Errores nuevos exportados: `ErrCostNotPositive`, `ErrNameTooShort`, `ErrItemInactive`,
`ErrItemNotFound`, `ErrAmountNotPositive`, `ErrInsufficientBalance`,
`ErrRedemptionNotFound`, `ErrNotTheRedeemer`, `ErrRedemptionVoided`.

### 10.2 Puertos

```go
// puerto de entrada (extiende, no rompe, la superficie existente)
type CatalogUseCase interface {
    CreateItem(ctx, cmd CreateItemCommand) (domain.CatalogItem, error)
    EditItem(ctx, cmd EditItemCommand) (domain.CatalogItem, error)
    ListItems(ctx, onlyActive bool) ([]domain.CatalogItem, error)
}
type RedemptionUseCase interface {
    Redeem(ctx, cmd RedeemCommand) (domain.Redemption, error)
    Void(ctx, id domain.RedemptionID, by domain.MemberID) (domain.Redemption, error)
    List(ctx, f RedemptionFilter) ([]domain.Redemption, error)
}

// puertos de salida
type CatalogRepository interface {
    Save(ctx, domain.CatalogItem) error
    Update(ctx, domain.CatalogItem) error
    FindByID(ctx, domain.ItemID) (domain.CatalogItem, error)
    List(ctx, onlyActive bool) ([]domain.CatalogItem, error)
}
type RedemptionRepository interface {
    // RedeemTx corre en una transacción: bloquea al titular, recalcula el disponible,
    // y solo si amount ≤ disponible inserta el canje. Devuelve ErrInsufficientBalance
    // si no alcanza. Aquí vive la garantía anti-sobregiro (RD-CAN-2).
    RedeemTx(ctx, r domain.Redemption, amount int) error
    UpdateStatus(ctx, domain.Redemption) error
    FindByID(ctx, domain.RedemptionID) (domain.Redemption, error)
    List(ctx, RedemptionFilter) ([]domain.Redemption, error)
    AvailableFor(ctx, domain.MemberID) (int, error) // fuera de tx, para lecturas
}
```

### 10.3 Servicio

`RedemptionService.Redeem`: busca el plan → si inactivo, `ErrItemInactive` → construye
el `Redemption` con el dominio (congela el costo) → `repo.RedeemTx(...)` que dentro de la
transacción verifica el saldo. El servicio no calcula el saldo fuera de la transacción
para decidir: eso sería una carrera. La verificación autoritativa es transaccional.

### 10.4 Adaptador HTTP

| Método | Ruta | operationId | Spec |
|---|---|---|---|
| POST | `/api/v1/catalog-items` | `createCatalogItem` | 006 |
| GET | `/api/v1/catalog-items` | `listCatalogItems` | 006 |
| PATCH | `/api/v1/catalog-items/{id}` | `editCatalogItem` | 006 |
| POST | `/api/v1/redemptions` | `redeem` | 007 |
| GET | `/api/v1/redemptions` | `listRedemptions` | 007 |
| POST | `/api/v1/redemptions/{id}/void` | `voidRedemption` | 007 |

Mapeo de error → status (se agrega al mapa existente):

```go
domain.ErrCostNotPositive:     http.StatusUnprocessableEntity, // 422
domain.ErrNameTooShort:        http.StatusUnprocessableEntity, // 422
domain.ErrAmountNotPositive:   http.StatusUnprocessableEntity, // 422
domain.ErrInsufficientBalance: http.StatusUnprocessableEntity, // 422
domain.ErrItemInactive:        http.StatusConflict,            // 409
domain.ErrRedemptionVoided:    http.StatusConflict,            // 409
domain.ErrNotTheRedeemer:      http.StatusForbidden,           // 403
domain.ErrItemNotFound:        http.StatusNotFound,            // 404
domain.ErrRedemptionNotFound:  http.StatusNotFound,            // 404
```

### 10.5 Plan de pruebas (fase 2)

- `domain/catalog_test.go` — `TierFor` en los bordes 5/6/50/51, costo/nombre inválidos.
- `domain/redemption_test.go` — amount, plan inactivo, congelado, anular/reanular, autor.
- `app/service/redemption_test.go` — CA-1..CA-6 con MemoryRepo (incluye disponible).
- `adapter/out/postgres` (integration) — columna `tier` generada, congelar precio,
  vista disponible, **sobregiro concurrente** (dos goroutines, una debe fallar).
- `adapter/in/http` — status por error, contrato de canje y catálogo.
- E2E — abonar → canjear → anular canje → el disponible vuelve.

### 10.6 Orden de implementación

1. Spec 006: dominio catálogo (`TierFor` primero, en rojo) → puerto → repo → handler.
2. Spec 005-enmienda: vista y saldo disponible.
3. Spec 007: dominio canje → servicio con `RedeemTx` transaccional → repo Postgres con
   `SELECT FOR UPDATE` → handler. El test de sobregiro concurrente es el que manda.
4. E2E y pulido.