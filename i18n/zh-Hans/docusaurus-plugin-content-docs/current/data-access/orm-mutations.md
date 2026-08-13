---
sidebar_position: 7
---

# ORM：写入操作

使用查询构造器写入数据：INSERT、UPDATE、DELETE、原始 SQL 与软删除行为。这些操作的事务化执行方式见[事务](./transactions)。

## INSERT 子句

```go
// 插入单条记录
user := &User{Username: "alice", Email: "alice@example.com"}
_, err := db.NewInsert().Model(user).Exec(ctx)

// 使用 RETURNING 插入（PostgreSQL）
err := db.NewInsert().Model(user).ReturningAll().Scan(ctx)

// 仅插入指定列
_, err := db.NewInsert().Model(user).
	Select("username", "email").
	Exec(ctx)

// 排除列
_, err := db.NewInsert().Model(user).
	Exclude("password").
	Exec(ctx)

// 显式设置列值
_, err := db.NewInsert().Model(user).
	Column("status", "active").
	ColumnExpr("score", func(eb orm.ExprBuilder) any {
		return eb.Literal(100)
	}).
	Exec(ctx)
```

### ON CONFLICT（Upsert）

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

## UPDATE 子句

```go
// 通过主键更新模型
user.Email = "new@example.com"
_, err := db.NewUpdate().Model(user).WherePK().Exec(ctx)

// 更新指定列
_, err := db.NewUpdate().Model(user).
	Select("email", "updated_at").
	WherePK().Exec(ctx)

// 显式设置值
_, err := db.NewUpdate().Model((*User)(nil)).
	Set("status", "inactive").
	SetExpr("updated_at", func(eb orm.ExprBuilder) any {
		return eb.Now()
	}).
	Where(func(cb orm.ConditionBuilder) {
		cb.Equals("status", "active").
			CreatedAtLessThan(cutoffTime)
	}).Exec(ctx)

// 忽略零值
_, err := db.NewUpdate().Model(user).OmitZero().WherePK().Exec(ctx)

// 批量更新
_, err := db.NewUpdate().Model(&users).Bulk().Exec(ctx)

// 带 RETURNING 的更新
err := db.NewUpdate().Model(user).WherePK().ReturningAll().Scan(ctx)
```

> 框架会在每种 UPDATE 形态上自动打上 `updated_at` 和 `updated_by`——基于模型的、`Set`/`SetExpr` 的、有列白名单的、批量更新均适用。`created_at` 和 `created_by` 会被自动从 UPDATE 中排除，以保护创建审计数据。显式 `Set("updated_at", ...)` 会被尊重——框架不会覆盖显式设置的审计列。

## DELETE 子句

```go
// 通过主键删除
_, err := db.NewDelete().Model(user).WherePK().Exec(ctx)

// 条件删除
_, err := db.NewDelete().Model((*User)(nil)).
	Where(func(cb orm.ConditionBuilder) {
		cb.Equals("status", "deactivated").
			CreatedAtLessThan(oneYearAgo)
	}).Exec(ctx)

// 强制删除（跳过软删除）
_, err := db.NewDelete().Model(user).WherePK().ForceDelete().Exec(ctx)

// 带 RETURNING 的删除
err := db.NewDelete().Model(user).WherePK().ReturningAll().Scan(ctx)
```

## MERGE（带源的 Upsert）

`MergeQuery` 执行 SQL `MERGE`，将目标表与源数据集同步。它仅在 PostgreSQL 上受支持；在其他数据库方言上，需使用等效的 insert-on-conflict 模式替代。

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

源数据形式：

- `Using(model, alias)` — 使用模型/表作为源。
- `UsingTable(name, alias)` — 使用已有表或 CTE。
- `UsingExpr(builder, alias)` / `UsingSubQuery(builder, alias)` — 使用子查询或表达式；默认别名为 `src`，可覆盖。

`WhenMatched`、`WhenNotMatched`、`WhenNotMatchedByTarget` 和
`WhenNotMatchedBySource` 均返回一个 `MergeWhenBuilder`，支持
`ThenUpdate`、`ThenInsert`、`ThenDelete` 和 `ThenDoNothing` 动作。

```go
// 仅在标志不同时更新
WhenMatched().
	ThenUpdate(func(ub orm.MergeUpdateBuilder) {
		ub.SetColumns("email").
			SetExpr("updated_at", func(eb orm.ExprBuilder) any { return eb.Now() })
	})

// 仅当版本不一致时才更新
WhenMatched(func(cb orm.ConditionBuilder) {
	cb.NotEqualsColumn("u.version", "src.version")
}).ThenUpdate(func(ub orm.MergeUpdateBuilder) {
	ub.SetColumns("name", "email")
})

// 插入目标中不存在的源行
WhenNotMatched().ThenInsert(func(ib orm.MergeInsertBuilder) {
	ib.ValuesAll("id") // id 自动生成，不从源复制
})
```

`MergeUpdateBuilder` 提供 `Set`、`SetExpr`、`SetColumns` 和 `SetAll`。
`SetAll` 从源复制每一列；传入要排除的列名（如 `"id"`）可跳过这些列。
`MergeInsertBuilder` 提供 `Value`、`ValueExpr`、`Values` 和 `ValuesAll`，
同样支持排除列。

`Returning` / `ReturningAll` / `ReturningNone` 的用法与其他写入查询一致。

## 原始 SQL 查询

```go
// 带参数绑定的原始 SQL
var result []MyStruct
db.NewRaw("SELECT * FROM users WHERE status = ?", "active").Scan(ctx, &result)
```

## 软删除支持

```go
// 仅查询已软删除的记录
db.NewSelect().Model(&users).WhereDeleted().Scan(ctx)

// 包含已软删除的记录
db.NewSelect().Model(&users).IncludeDeleted().Scan(ctx)
```

## 下一步

- [事务](./transactions) — 使用 `RunInTx` 原子化地执行写入操作
- [ORM：DDL 与 Surface Map](./orm-ddl) — DDL 构造器与 `orm` 包完整公开接口面
