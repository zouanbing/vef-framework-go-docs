---
sidebar_position: 7
---

# ORM: Mutations

Writing data with the query builder: INSERT, UPDATE, DELETE, raw queries, and soft-delete behavior. For transactional execution of these operations, see [Transactions](./transactions).

## INSERT Clause

```go
// Insert a single record
user := &User{Username: "alice", Email: "alice@example.com"}
_, err := db.NewInsert().Model(user).Exec(ctx)

// Insert with RETURNING (PostgreSQL)
err := db.NewInsert().Model(user).ReturningAll().Scan(ctx)

// Insert specific columns only
_, err := db.NewInsert().Model(user).
	Select("username", "email").
	Exec(ctx)

// Exclude columns
_, err := db.NewInsert().Model(user).
	Exclude("password").
	Exec(ctx)

// Set column values explicitly
_, err := db.NewInsert().Model(user).
	Column("status", "active").
	ColumnExpr("score", func(eb orm.ExprBuilder) any {
		return eb.Literal(100)
	}).
	Exec(ctx)
```

### ON CONFLICT (Upsert)

```go
// ON CONFLICT (username) DO UPDATE SET email = EXCLUDED.email
_, err := db.NewInsert().Model(user).
	OnConflict(func(cb orm.ConflictBuilder) {
		cb.Columns("username").
			DoUpdate().
			Set("email", user.Email)
	}).Exec(ctx)

// ON CONFLICT DO NOTHING
_, err := db.NewInsert().Model(user).
	OnConflict(func(cb orm.ConflictBuilder) {
		cb.Columns("username").DoNothing()
	}).Exec(ctx)
```

## UPDATE Clause

```go
// Update a model by PK
user.Email = "new@example.com"
_, err := db.NewUpdate().Model(user).WherePK().Exec(ctx)

// Update specific columns
_, err := db.NewUpdate().Model(user).
	Select("email", "updated_at").
	WherePK().Exec(ctx)

// Set values explicitly
_, err := db.NewUpdate().Model((*User)(nil)).
	Set("status", "inactive").
	SetExpr("updated_at", func(eb orm.ExprBuilder) any {
		return eb.Now()
	}).
	Where(func(cb orm.ConditionBuilder) {
		cb.Equals("status", "active").
			CreatedAtLessThan(cutoffTime)
	}).Exec(ctx)

// Omit zero values
_, err := db.NewUpdate().Model(user).OmitZero().WherePK().Exec(ctx)

// Bulk update
_, err := db.NewUpdate().Model(&users).Bulk().Exec(ctx)

// Update with RETURNING
err := db.NewUpdate().Model(user).WherePK().ReturningAll().Scan(ctx)
```

> The framework stamps `updated_at` and `updated_by` on every UPDATE shape — model-based, `Set`/`SetExpr`, column whitelist, or bulk. `created_at` and `created_by` are automatically excluded from UPDATE to preserve creation audit data. An explicit `Set("updated_at", ...)` is respected — the framework does not overwrite an explicitly set audit column.

## DELETE Clause

```go
// Delete by PK
_, err := db.NewDelete().Model(user).WherePK().Exec(ctx)

// Delete with condition
_, err := db.NewDelete().Model((*User)(nil)).
	Where(func(cb orm.ConditionBuilder) {
		cb.Equals("status", "deactivated").
			CreatedAtLessThan(oneYearAgo)
	}).Exec(ctx)

// Force delete (bypass soft delete)
_, err := db.NewDelete().Model(user).WherePK().ForceDelete().Exec(ctx)

// Delete with RETURNING
err := db.NewDelete().Model(user).WherePK().ReturningAll().Scan(ctx)
```

## MERGE (Upsert with Source)

`MergeQuery` performs a SQL `MERGE` that synchronizes a target table with a
source data set. It is supported on PostgreSQL; on other dialects it either does
not apply or must be replaced with an an equivalent insert-on-conflict pattern.

```go
// MERGE INTO users AS u USING _source_data AS src ON u.id = src.id
// WHEN MATCHED THEN UPDATE SET name = src.name, ...
// WHEN NOT MATCHED THEN INSERT (id, name, ...) VALUES (src.id, src.name, ...)
_, err := db.NewMerge().
	Model(&User{}).
	WithValues("_source_data", &sourceUsers).
	UsingTable("_source_data").
	On(func(cb orm.ConditionBuilder) {
		cb.EqualsColumn("u.id", "_source_data.id")
	}).
	WhenMatched().
	ThenUpdate(func(ub orm.MergeUpdateBuilder) {
		ub.SetColumns("name", "email", "age", "is_active")
	}).
	WhenNotMatched().
	ThenInsert(func(ib orm.MergeInsertBuilder) {
		ib.Values("id", "name", "email", "age", "is_active")
	}).
	Exec(ctx)
```

Source forms:

- `Using(model, alias)` — use a model/table as the source.
- `UsingTable(name, alias)` — use an existing table or CTE.
- `UsingExpr(builder, alias)` / `UsingSubQuery(builder, alias)` — use a
  subquery or expression; the default alias is `src` unless overridden.

`WhenMatched`, `WhenNotMatched`, `WhenNotMatchedByTarget`, and
`WhenNotMatchedBySource` each return a `MergeWhenBuilder` with the actions
`ThenUpdate`, `ThenInsert`, `ThenDelete`, and `ThenDoNothing`.

```go
// Update only when a flag differs
WhenMatched().
	ThenUpdate(func(ub orm.MergeUpdateBuilder) {
		ub.SetColumns("email").
			SetExpr("updated_at", func(eb orm.ExprBuilder) any { return eb.Now() })
	})

// Skip rows that already look current
WhenMatched(func(cb orm.ConditionBuilder) {
	cb.NotEqualsColumn("u.version", "src.version")
}).ThenUpdate(func(ub orm.MergeUpdateBuilder) {
	ub.SetColumns("name", "email")
})

// Insert source rows not found in the target
WhenNotMatched().ThenInsert(func(ib orm.MergeInsertBuilder) {
	ib.ValuesAll("id") // id is auto-generated, do not copy from source
})
```

`MergeUpdateBuilder` provides `Set`, `SetExpr`, `SetColumns`, and `SetAll`.
`SetAll` copies every column from the source; pass columns to exclude, such as
`"id"`, to omit them. `MergeInsertBuilder` provides `Value`, `ValueExpr`,
`Values`, and `ValuesAll` with the same exclusion behavior.

`Returning` / `ReturningAll` / `ReturningNone` work the same as on other
mutation queries.

## Raw Queries

```go
// Raw SQL with parameter binding
var result []MyStruct
db.NewRaw("SELECT * FROM users WHERE status = ?", "active").Scan(ctx, &result)
```

## Soft Delete Support

```go
// Query only soft-deleted records
db.NewSelect().Model(&users).WhereDeleted().Scan(ctx)

// Include soft-deleted records
db.NewSelect().Model(&users).IncludeDeleted().Scan(ctx)
```

## Next Step

- [Transactions](./transactions) — running mutations atomically with `RunInTx`
- [ORM: DDL & Surface Map](./orm-ddl) — schema DDL builders and the complete public `orm` surface
