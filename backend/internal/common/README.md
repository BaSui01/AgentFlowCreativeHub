# Common 模块

## 概述

`common` 模块提供跨模块共享的基础结构和工具函数，包括软删除支持、时间戳管理和查询范围（Scopes）。

## 模块结构

```
common/
├── models.go   # 基础模型定义
└── scopes.go   # GORM 查询范围
```

---

## 📦 models.go - 基础模型

### SoftDeleteModel

软删除基础模型，提供统一的软删除字段和方法。

**字段**：
- `DeletedAt *time.Time` - 软删除时间，NULL 表示未删除
- `DeletedBy string` - 执行删除操作的用户ID

**方法**：
```go
// IsDeleted 检查记录是否已被软删除
func (m *SoftDeleteModel) IsDeleted() bool

// SoftDelete 执行软删除操作
func (m *SoftDeleteModel) SoftDelete(operatorID string)

// Restore 恢复已删除的记录
func (m *SoftDeleteModel) Restore()
```

**使用示例**：
```go
type User struct {
    ID       string `json:"id" gorm:"primaryKey"`
    Name     string `json:"name"`
    // ... 其他字段
    
    // 嵌入软删除模型
    common.SoftDeleteModel
}

// 软删除用户
user.SoftDelete("operator-user-id")
db.Save(&user)

// 检查是否已删除
if user.IsDeleted() {
    // 处理已删除的情况
}

// 恢复用户
user.Restore()
db.Save(&user)
```

---

### TimestampModel

时间戳基础模型，提供统一的创建时间和更新时间字段。

**字段**：
- `CreatedAt time.Time` - 创建时间，自动设置
- `UpdatedAt time.Time` - 更新时间，自动更新

**使用示例**：
```go
type Role struct {
    ID   string `json:"id" gorm:"primaryKey"`
    Name string `json:"name"`
    
    // 嵌入时间戳模型
    common.TimestampModel
}

// GORM 会自动管理 CreatedAt 和 UpdatedAt
```

---

### AuditableModel

可审计模型，结合时间戳和软删除功能。

**字段**：
- 包含 `TimestampModel` 的所有字段
- 包含 `SoftDeleteModel` 的所有字段

**使用示例**：
```go
type Tenant struct {
    ID   string `json:"id" gorm:"primaryKey"`
    Name string `json:"name"`
    
    // 嵌入可审计模型（包含时间戳和软删除）
    common.AuditableModel
}
```

---

## 🔍 scopes.go - GORM 查询范围

### NotDeleted

过滤已软删除的记录（默认查询行为）。

**使用示例**：
```go
// 查询未删除的用户
var users []User
db.Scopes(common.NotDeleted()).Find(&users)

// 组合使用
db.Scopes(
    common.NotDeleted(),
    common.ByTenant(tenantID),
).Find(&users)
```

---

### WithDeleted

包含已软删除的记录（查询所有记录）。

**使用示例**：
```go
// 查询所有用户（包括已删除）
var users []User
db.Scopes(common.WithDeleted()).Find(&users)
```

---

### OnlyDeleted

仅查询已软删除的记录。

**使用示例**：
```go
// 查询已删除的用户（回收站功能）
var deletedUsers []User
db.Scopes(common.OnlyDeleted()).Find(&deletedUsers)
```

---

### ByTenant

按租户ID过滤（多租户查询通用Scope）。

**使用示例**：
```go
// 查询指定租户的用户
var users []User
db.Scopes(common.ByTenant(tenantID)).Find(&users)

// 组合使用
db.Scopes(
    common.ByTenant(tenantID),
    common.NotDeleted(),
    common.ActiveOnly(),
).Find(&users)
```

---

### ActiveOnly

仅查询活跃状态的记录。

**使用示例**：
```go
// 查询活跃用户
var activeUsers []User
db.Scopes(common.ActiveOnly()).Find(&activeUsers)
```

---

## 🎯 最佳实践

### 1. 软删除使用规范

**DO**：
```go
// ✅ 使用软删除
user.SoftDelete(currentUserID)
db.Save(&user)

// ✅ 默认查询自动过滤已删除记录
db.Scopes(common.NotDeleted()).Find(&users)
```

**DON'T**：
```go
// ❌ 直接物理删除（除非确实需要）
db.Delete(&user)

// ❌ 忘记过滤已删除记录
db.Find(&users) // 会包含已删除记录
```

---

### 2. 查询范围组合使用

```go
// 推荐：组合多个 Scope
db.Scopes(
    common.ByTenant(tenantID),      // 租户隔离
    common.NotDeleted(),             // 过滤已删除
    common.ActiveOnly(),             // 仅活跃记录
).
Where("email LIKE ?", "%@example.com").
Order("created_at DESC").
Limit(10).
Find(&users)
```

---

### 3. 软删除恢复功能

```go
// 查询已删除的记录
var deletedUsers []User
db.Scopes(
    common.OnlyDeleted(),
    common.ByTenant(tenantID),
).Find(&deletedUsers)

// 恢复指定用户
for _, user := range deletedUsers {
    user.Restore()
    db.Save(&user)
}
```

---

### 4. 审计日志集成

```go
// 在 Service 层记录软删除操作
func (s *UserService) DeleteUser(userID string, operatorID string) error {
    var user User
    if err := s.db.First(&user, "id = ?", userID).Error; err != nil {
        return err
    }
    
    // 执行软删除
    user.SoftDelete(operatorID)
    
    // 保存并记录审计日志
    if err := s.db.Save(&user).Error; err != nil {
        return err
    }
    
    // 记录到审计日志（触发器会自动记录）
    s.auditLogger.Log(operatorID, "delete_user", "users", user.ID)
    
    return nil
}
```

---

## 🔧 数据库触发器支持

配合 `db/migrations/0004_add_triggers.sql` 使用，自动实现：

1. **自动更新 updated_at**：任何表更新时自动设置
2. **统计字段维护**：知识库文档数、分片数自动更新
3. **软删除审计**：软删除操作自动记录到审计日志

---

## 📝 注意事项

### 性能考虑

1. **使用部分索引**：
   ```sql
   CREATE INDEX idx_users_deleted_at 
       ON users(deleted_at) 
       WHERE deleted_at IS NULL;
   ```
   仅索引未删除记录，提升查询性能。

2. **避免全表扫描**：
   ```go
   // ❌ 不要这样
   db.Find(&users)
   
   // ✅ 应该这样
   db.Scopes(common.NotDeleted()).Find(&users)
   ```

### 外键约束

软删除可能导致外键引用已删除记录，建议：

- 审计日志等使用 `ON DELETE SET NULL`
- 其他关联使用软删除级联

---

## 🚀 迁移指南

### 从旧模型迁移

如果现有代码使用了旧的模型定义，迁移步骤：

1. **运行迁移脚本**：
   ```bash
   # 应用软删除字段
   psql -d your_db -f db/migrations/0003_add_soft_delete.sql
   
   # 添加触发器
   psql -d your_db -f db/migrations/0004_add_triggers.sql
   ```

2. **更新 Go 模型**：
   ```go
   // 旧模型
   type User struct {
       ID        string
       Name      string
       CreatedAt time.Time
       UpdatedAt time.Time
   }
   
   // 新模型（嵌入 common 基础模型）
   type User struct {
       ID   string `gorm:"primaryKey"`
       Name string
       common.AuditableModel // 包含时间戳和软删除
   }
   ```

3. **更新查询代码**：
   ```go
   // 旧代码
   db.Where("tenant_id = ?", tenantID).Find(&users)
   
   // 新代码（添加软删除过滤）
   db.Scopes(
       common.ByTenant(tenantID),
       common.NotDeleted(),
   ).Find(&users)
   ```

---

## 📚 相关文档

- [数据库设计文档](../../../docs/数据库设计文档.md)
- [数据模型完善性分析](../../../.factory/docs/2025-11-17-spec.md)
- [迁移脚本](../../../db/migrations/)
