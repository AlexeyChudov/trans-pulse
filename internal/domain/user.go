package domain

import "time"

type Role string

const (
	RoleAdmin      Role = "ADMIN"
	RoleDispatcher Role = "DISPATCHER"
)

func (r Role) IsValid() bool { return r == RoleAdmin || r == RoleDispatcher }

type User struct {
	ID           int64
	Login        string
	PasswordHash string
	Role         Role
	IsActive     bool
	CreatedAt    time.Time
}
