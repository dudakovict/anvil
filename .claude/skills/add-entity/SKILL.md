---
name: add-entity
description: Add a new domain entity through every layer of the hexagon — domain, port, service, migration, postgres repository, REST handler, wiring. Use when adding an entity, resource or CRUD endpoint set.
---

# Adding an entity

Follow the `user` files as the reference implementation at every step. The
example below uses `product` — substitute your entity. Load the `go-tests`
skill for the test files.

## 1. Domain — `internal/core/domain/product.go`

`var` error block at the top, then the entity and filter:

```go
var (
	ErrProductNotFound = errors.New("product not found")
	ErrInvalidProduct  = errors.New("invalid product")
)

type Product struct {
	ID        int64
	Name      string
	CreatedAt time.Time
	UpdatedAt time.Time
}

type ProductFilter struct {
	Name *string
}
```

## 2. Migration — `make migrate-create name=create_products`

`bigint GENERATED ALWAYS AS IDENTITY` primary key (never random UUIDs),
`timestamptz` defaults, constraints in SQL (UNIQUE, CHECK) — copy
`migrations/00001_create_users.sql`.

## 3. Port — `internal/core/port/product.go`

`ProductService` (in-port) and `ProductRepository` (out-port) interfaces.
Method order: Create, Get, List, Update, Delete. Then **append both to the
mockgen line in the Makefile** and run `make mocks`.

## 4. Service — `internal/core/service/product.go`

Business logic + validation that must not trust adapters. Include the
interface assertion and constructor:

```go
var _ port.ProductService = (*ProductService)(nil)

func NewProductService(repo port.ProductRepository) *ProductService {
	return &ProductService{repo: repo}
}
```

Add `product_test.go` (gomock table tests — see the `go-tests` skill).

## 5. Postgres — `internal/adapter/postgres/product.go` + row in `model.go`

- `productRow` struct + `TableName()` + `toProductRow`/`toDomain` converters
  in `model.go`
- Repository methods copy the `user.go` shapes exactly, including error
  classification: `gorm.ErrRecordNotFound` → `domain.ErrProductNotFound`,
  `gorm.ErrDuplicatedKey` → the domain conflict error,
  `RowsAffected == 0` → not found on update/delete,
  `Clauses(clause.Returning{})` so timestamps come from the database
- `List` uses the `filtered()` closure + `Count` + `order.SQL("id")` +
  `Limit/Offset`
- Integration tests go in `product_test.go` with a `seedProduct` helper; add
  the table to the TRUNCATE in `newTestRepo`

## 6. REST — `internal/adapter/rest/product.go` + DTOs in `dto.go`

- DTOs: `CreateProductRequest` etc. with `validate` tags (checked
  automatically by `web.Decode`; add a `Validate() error` method only for
  cross-field rules); response struct without internal fields; converters
  `toProductResponse(s)`
- For list endpoints: a swagger alias `type ProductsPage = query.Result[ProductResponse]`
  and a `productSortFields` whitelist; rich filter structs can use
  `web.DecodeQuery[T]` with `form` tags instead of reading params by hand
- Handlers use the `web.Handler` signature with swagger annotations — copy a
  `user.go` handler including the `@Failure` set (400/401/403/404/409/500 as
  applicable) and `@Security BearerAuth` on protected ones
- Extend `errorResponse` in `user.go` (or a shared switch) with the new
  domain errors → statuses
- Routes: split into `publicRoutes`/`protectedRoutes`/`adminRoutes` methods
  and mount them in `router.go` in the matching groups; decide which routes
  need ownership checks (`canAccess` pattern) vs role gating

## 7. Wire — `cmd/api/main.go`

```go
products := service.NewProductService(postgres.NewProductRepository(db))
```

and add the field to `rest.Deps` + pass it in `NewRouter`.

## Verify

`make mocks && make swagger && make fmt && make lint && make test`, then e2e
through compose (`make docker-up`, curl the new endpoints, check a trace in
Grafana). Update the README architecture tree only if you added a package.
