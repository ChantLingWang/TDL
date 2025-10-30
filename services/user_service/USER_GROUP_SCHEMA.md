# 用户-组群关系数据模型设计

## 表结构设计

### 1. 用户表 (users)
```sql
CREATE TABLE users (
    user_id VARCHAR(50) PRIMARY KEY,
    username VARCHAR(100) NOT NULL,
    email VARCHAR(255) UNIQUE NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
```

### 2. 组群表 (groups)
```sql
CREATE TABLE groups (
    group_id VARCHAR(50) PRIMARY KEY,
    group_name VARCHAR(100) NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
```

### 3. 用户组群关系表 (user_groups)
```sql
CREATE TABLE user_groups (
    user_id VARCHAR(50) REFERENCES users(user_id) ON DELETE CASCADE,
    group_id VARCHAR(50) REFERENCES groups(group_id) ON DELETE CASCADE,
    joined_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (user_id, group_id)
);
```

## 常用查询示例

### 1. 查询用户所属的所有组群
```sql
SELECT g.group_id, g.group_name 
FROM groups g
JOIN user_groups ug ON g.group_id = ug.group_id
WHERE ug.user_id = $1;
```

### 2. 查询组群内的所有用户
```sql
SELECT u.user_id, u.username, u.email
FROM users u
JOIN user_groups ug ON u.user_id = ug.user_id
WHERE ug.group_id = $1;
```

### 3. 添加用户到组群
```sql
INSERT INTO user_groups (user_id, group_id) 
VALUES ($1, $2) 
ON CONFLICT (user_id, group_id) DO NOTHING;
```

### 4. 从组群移除用户
```sql
DELETE FROM user_groups 
WHERE user_id = $1 AND group_id = $2;
```

## GORM 模型定义示例

```go
type User struct {
    UserID    string    `gorm:"primaryKey;column:user_id;type:varchar(50)"`
    Username  string    `gorm:"column:username;type:varchar(100)"`
    Email     string    `gorm:"column:email;type:varchar(255);uniqueIndex"`
    CreatedAt time.Time `gorm:"column:created_at"`
    UpdatedAt time.Time `gorm:"column:updated_at"`
}

type Group struct {
    GroupID   string    `gorm:"primaryKey;column:group_id;type:varchar(50)"`
    GroupName string    `gorm:"column:group_name;type:varchar(100)"`
    CreatedAt time.Time `gorm:"column:created_at"`
    UpdatedAt time.Time `gorm:"column:updated_at"`
}

type UserGroup struct {
    UserID   string `gorm:"primaryKey;column:user_id;type:varchar(50)"`
    GroupID  string `gorm:"primaryKey;column:group_id;type:varchar(50)"`
    JoinedAt time.Time `gorm:"column:joined_at"`
}

// 设置表名
func (User) TableName() string {
    return "users"
}

func (Group) TableName() string {
    return "groups"
}

func (UserGroup) TableName() string {
    return "user_groups"
}
```