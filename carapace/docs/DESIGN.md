# Carapace（甲壳）— 虫族密钥管理基础库

> 版本: v0.1 Draft
> 日期: 2026-03-22
> 状态: 设计阶段

## 一、定位

Carapace 是 StarClaw 虫族生态的**闭源共享安全库**，为所有闭源组件（Hive、Queen、Overlord、Nydus）提供统一的敏感信息保护能力。

**不是独立运行的服务**，而是一个 Go module，被各组件 `import` 引用。

```
              ┌─────────────────────────┐
              │    Carapace (Go Lib)    │
              │  加密 · 密钥管理 · 审计   │
              └──────────┬──────────────┘
                         │ import
          ┌──────────────┼──────────┬────────────┐
          ▼              ▼          ▼            ▼
        Hive          Queen     Overlord      Nydus
      (托管平台)      (总控)     (企业管控)    (隧道)
```

**Claw（开源）** 不直接依赖 Carapace，保留 `security/crypto.go` 作为轻量内嵌版，但**格式兼容**（`enc:` 前缀）。

---

## 二、核心概念

### 2.1 信封加密（Envelope Encryption）

```
┌─────────────────────────────────────────────────────────┐
│  Master Key (MK)                                         │
│  来源: 环境变量 / 文件 / 阿里云 KMS / AWS KMS             │
│  作用: 仅用于加密/解密 Data Key                           │
│  特点: 永远不接触实际数据                                  │
└─────────────────────────┬───────────────────────────────┘
                          │ 加密保护
                          ▼
┌─────────────────────────────────────────────────────────┐
│  Data Key (DK) — 版本化                                   │
│  DK_v1, DK_v2, DK_v3...                                  │
│  存储: DB 表 `carapace_keys`，被 MK 加密后存储             │
│  作用: 实际加密数据                                        │
│  轮转: 新建 DK_vN，旧版本保留（只解密不加密）               │
└─────────────────────────┬───────────────────────────────┘
                          │ 加密保护
                          ▼
┌─────────────────────────────────────────────────────────┐
│  Secrets (实际敏感数据)                                    │
│  API Key, DB Password, JWT Secret, Token...               │
│  存储格式: enc:v{N}:{base64(nonce+ciphertext+tag)}        │
└─────────────────────────────────────────────────────────┘
```

**为什么信封加密？**
- 轮转 Master Key → 只需重新加密 Data Keys（几十个），不用重新加密所有数据（几万条）
- 轮转 Data Key → 新数据用新 key，旧数据按需重新加密（可渐进式）
- Master Key 可以放在 KMS 中，永远不落盘

### 2.2 密文格式

```
Phase 1-2（当前）:  enc:{base64}
Phase 3（Carapace）: enc:v{version}:{base64}
```

**向后兼容规则：**
- `enc:v2:xxx` → Carapace 信封加密，用 DK_v2 解密
- `enc:xxx`    → 旧格式，用 master key 直接解密（Phase 1-2 兼容）
- `sk-xxx...`  → 明文，直接返回（迁移兼容）

### 2.3 Purpose 隔离

每个用途派生独立密钥，即使同一 Data Key 也无法交叉解密：

```
DK_v1 + "api_key"     → 派生密钥 A → 加密 API Key
DK_v1 + "db_password"  → 派生密钥 B → 加密 DB 密码
DK_v1 + "jwt_secret"   → 派生密钥 C → 加密 JWT Secret
```

---

## 三、接口设计

### 3.1 核心 Vault 接口

```go
package carapace

// Vault is the main interface for secret management.
type Vault interface {
    // Seal encrypts a plaintext secret for storage.
    // Returns formatted ciphertext: "enc:v{N}:{base64}"
    Seal(purpose string, plaintext string) (string, error)

    // Unseal decrypts a stored secret.
    // Handles all formats: enc:vN:, enc:, and plaintext (migration).
    Unseal(purpose string, ciphertext string) (string, error)

    // RotateDataKey creates a new data key version.
    // Old keys are retained for decryption only.
    RotateDataKey() (version int, error)

    // RotateMasterKey re-encrypts all data keys with a new master key.
    RotateMasterKey(newMasterKey []byte) error

    // ReEncrypt re-encrypts a value with the current (latest) data key.
    // Used for gradual migration of old-format secrets.
    ReEncrypt(purpose string, ciphertext string) (string, error)

    // Info returns vault status (key versions, backend, fingerprint).
    Info() VaultInfo
}

type VaultInfo struct {
    Backend           string // "env", "file", "kms"
    MasterFingerprint string // safe identifier
    CurrentKeyVersion int
    TotalKeyVersions  int
    Initialized       bool
}
```

### 3.2 后端接口

```go
// MasterKeyBackend provides the master key from various sources.
type MasterKeyBackend interface {
    // LoadMasterKey returns the 32-byte master key.
    LoadMasterKey() ([]byte, error)

    // Name returns the backend identifier.
    Name() string
}
```

内置实现：

| 后端 | 文件 | 说明 |
|------|------|------|
| `EnvBackend` | `backend/env.go` | 从环境变量读取（当前模式） |
| `FileBackend` | `backend/file.go` | 从加密文件读取（多 key 支持） |
| `AliyunKMSBackend` | `backend/aliyun_kms.go` | 阿里云 KMS（Phase 4） |

### 3.3 Data Key 存储

```sql
CREATE TABLE carapace_keys (
    version       INT PRIMARY KEY AUTO_INCREMENT,
    encrypted_key VARCHAR(500) NOT NULL,  -- base64(MK加密后的DK)
    purpose       VARCHAR(50) DEFAULT 'default',
    status        ENUM('active', 'decrypt_only', 'retired') DEFAULT 'active',
    created_at    TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    rotated_at    TIMESTAMP NULL
);
```

- `active`: 当前版本，用于加密和解密
- `decrypt_only`: 旧版本，仅用于解密历史数据
- `retired`: 已废弃，所有数据已迁移到新版本

### 3.4 审计日志

```go
// AuditEntry records a secret access event.
type AuditEntry struct {
    Timestamp  time.Time
    Operation  string // "seal", "unseal", "rotate", "re-encrypt"
    Purpose    string // "api_key", "db_password", etc.
    KeyVersion int
    Caller     string // service name: "hive", "queen", etc.
    Success    bool
    Error      string
}
```

---

## 四、目录结构

```
carapace/
├── go.mod
├── docs/
│   └── DESIGN.md          # 本文档
├── vault.go               # Vault 接口 + 默认实现
├── aes.go                 # AES-256-GCM 加密/解密
├── kdf.go                 # HKDF 密钥派生
├── envelope.go            # 信封加密：MK→DK→数据
├── format.go              # 密文格式解析：enc:, enc:vN:
├── rotation.go            # 密钥轮转逻辑
├── audit.go               # 审计日志记录
├── migrate.go             # 数据迁移工具
├── backend/
│   ├── env.go             # 环境变量后端
│   ├── file.go            # 文件后端
│   └── aliyun_kms.go      # 阿里云 KMS 后端（stub）
└── carapace_test.go       # 测试
```

---

## 五、使用方式

### 5.1 Hive 集成示例

```go
import "github.com/yinhe/starclaw/carapace"
import "github.com/yinhe/starclaw/carapace/backend"

// 启动时初始化
vault, err := carapace.NewVault(carapace.Config{
    Backend: backend.NewEnvBackend("HIVE_MASTER_KEY"),
    DB:      db, // GORM DB for data key storage
    Service: "hive",
})

// 加密（写入 DB 前）
inst.DBPassword, _ = vault.Seal("db_password", plainPassword)
inst.JWTSecret, _ = vault.Seal("jwt_secret", plainJWT)

// 解密（从 DB 读出后）
plainPassword, _ = vault.Unseal("db_password", inst.DBPassword)
plainJWT, _ = vault.Unseal("jwt_secret", inst.JWTSecret)

// 密钥轮转（运维操作）
newVersion, _ := vault.RotateDataKey()
log.Printf("Rotated to data key v%d", newVersion)
```

### 5.2 Claw 兼容

Claw 的 `security/crypto.go` 保持不变，但格式兼容：

```go
// Claw 产生: enc:{base64}
// Carapace 能解密 ✓

// Carapace 产生: enc:v2:{base64}
// Claw 无法解密 ✗（但 Claw 不需要解密其他组件的数据）
```

这是**有意为之**：Claw 是开源的，只管自己的数据；闭源组件用 Carapace 获得更强保护。

---

## 六、迁移路径

### Phase 1（已完成）：本地加密
- Hive: `hive/service/crypto.go` — AES-256-GCM + `HIVE_MASTER_KEY`
- 格式: `enc:{base64}`

### Phase 2（已完成）：Claw API Key 加密
- Claw: `security/crypto.go` — AES-256-GCM + `STARCLAW_MASTER_KEY`
- `EncryptAPIKey()` / `DecryptAPIKey()` 全局函数
- 格式: `enc:{base64}`

### Phase 3（本文档）：Carapace 共享库
- 创建 `carapace/` 顶层模块
- 信封加密 + Data Key 版本化
- Hive/Queen/Overlord 迁移到 Carapace
- 格式: `enc:v{N}:{base64}`
- **向后兼容** Phase 1-2 的 `enc:{base64}` 格式

### Phase 4（未来）：KMS 集成
- 阿里云 KMS 作为 Master Key 后端
- Master Key 永远不落盘
- 适用于合规要求高的企业部署

### Phase 5（未来）：动态密钥
- 临时数据库凭证（类似 Vault dynamic secrets）
- 自动过期 + 自动续期
- 仅在 > 10000 节点规模时考虑

---

## 七、安全设计原则

1. **最小权限**: 每个组件只能解密自己 purpose 的数据
2. **密钥分离**: Master Key ≠ Data Key ≠ 派生密钥，三层隔离
3. **向前保密**: 轮转后旧 key 不能加密新数据
4. **审计可追溯**: 每次加密/解密操作记录审计日志
5. **优雅降级**: KeyManager 不可用时，服务不崩溃，只是无法解密
6. **零停机轮转**: 密钥轮转不需要重启服务
7. **格式自描述**: 密文自带版本号，无需外部状态即可确定解密方式

---

## 八、开闭源边界

| 组件 | 加密方式 | 依赖 |
|------|----------|------|
| **Claw** (开源) | `security/crypto.go` 本地版 | 无外部依赖 |
| **Hive** (闭源) | `carapace.Vault` | `import carapace` |
| **Queen** (闭源) | `carapace.Vault` | `import carapace` |
| **Overlord** (闭源) | `carapace.Vault` | `import carapace` |
| **Nydus** (闭源) | `carapace.Vault` | `import carapace` |

Claw 的 `EncryptAPIKey`/`DecryptAPIKey` 与 Carapace 的 `Seal`/`Unseal` **格式兼容**，
但 Claw 只处理 `enc:` 格式，Carapace 处理 `enc:` 和 `enc:vN:` 两种格式。

---

## 九、何时实现？

**触发条件（满足任一即启动）：**
- Hive 管理 > 50 个实例（密钥集中度高）
- 企业客户要求密钥轮转能力
- 需要接入阿里云 KMS
- 发生安全事件需要紧急轮转所有密钥

**当前优先级：低**（Phase 1-2 的本地加密已覆盖核心需求）
